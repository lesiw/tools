// Package uselessconcat provides an analysis.Analyzer that reports string
// literals joined by + where the join fits on one line.
//
// A message split across lines to stay under the limit is a normal way to
// write a long string. The + is what makes the split possible, so it earns its
// place while the line needs it and becomes noise the moment the line fits
// without it. Nothing marks that moment: a rename, a shorter message, or a
// formatter run that rejoined the operands leaves the + behind, and the reader
// sees two literals where one would do.
//
// The analyzer reports the join only when the merged literal keeps the line
// within the limit, so a split that is doing work stays.
//
// The fix is withheld when an operand is a raw string holding a newline: the
// merged literal would have to be escaped to fit on one line, which changes
// how the string reads rather than how it is written.
package uselessconcat

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "uselessconcat",
	Doc:  "reports string literals joined by + where the join fits on one line",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			defer func() { stack = append(stack, n) }()
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || bin.Op != token.ADD {
				return true
			}
			// A chain is one expression. Only its outermost + is
			// reported, so a run of three literals is one finding
			// rather than three.
			if len(stack) > 0 {
				parent, ok := stack[len(stack)-1].(*ast.BinaryExpr)
				if ok && parent.Op == token.ADD {
					return true
				}
			}
			check(pass, bin)
			return true
		})
	}
	return nil, nil
}

// check reports bin when it joins string literals that would still fit on one
// line as a single literal.
func check(pass *analysis.Pass, bin *ast.BinaryExpr) {
	operands, ok := flatten(bin)
	if !ok || len(operands) < 2 {
		return
	}
	merged, ok := merge(operands)
	if !ok {
		return
	}
	if !fits(pass, bin, merged) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: "string literals joined by + fit on one line",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "merge the literals",
			TextEdits: []analysis.TextEdit{{
				Pos:     bin.Pos(),
				End:     bin.End(),
				NewText: []byte(merged),
			}},
		}},
	})
}

// flatten returns the operands of a chain of + expressions, and whether every
// one of them is a string literal.
func flatten(bin *ast.BinaryExpr) ([]*ast.BasicLit, bool) {
	var (
		out  []*ast.BasicLit
		ok   = true
		walk func(e ast.Expr)
	)
	walk = func(e ast.Expr) {
		if b, isBin := e.(*ast.BinaryExpr); isBin && b.Op == token.ADD {
			walk(b.X)
			walk(b.Y)
			return
		}
		lit, isLit := e.(*ast.BasicLit)
		if !isLit || lit.Kind != token.STRING {
			ok = false
			return
		}
		out = append(out, lit)
	}
	walk(bin)
	return out, ok
}

// merge returns the source text of the operands joined into one literal, and
// whether such a literal can be written at all.
//
// The merged literal keeps the kind the operands share: raw strings stay raw,
// interpreted ones stay interpreted. A mix has no single spelling, so it is
// left alone.
func merge(operands []*ast.BasicLit) (string, bool) {
	var (
		value strings.Builder
		raw   = strings.HasPrefix(operands[0].Value, "`")
	)
	for _, lit := range operands {
		if strings.HasPrefix(lit.Value, "`") != raw {
			return "", false
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return "", false
		}
		value.WriteString(v)
	}
	if raw {
		// A raw string cannot hold a backtick, and one holding a
		// newline is a block rather than a line.
		if strings.ContainsAny(value.String(), "`\n") {
			return "", false
		}
		return "`" + value.String() + "`", true
	}
	if strings.ContainsRune(value.String(), '\n') {
		return "", false
	}
	return strconv.Quote(value.String()), true
}

// fits reports whether replacing bin with merged is safe. A join on one line
// can only shorten it; a join across lines needs a width to say the result
// fits.
func fits(pass *analysis.Pass, bin *ast.BinaryExpr, merged string) bool {
	var (
		start = pass.Fset.Position(bin.Pos())
		end   = pass.Fset.Position(bin.End())
	)
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	lines := strings.Split(string(src), "\n")
	if start.Line < 1 || start.Line > len(lines) {
		return false
	}
	if end.Line < 1 || end.Line > len(lines) {
		return false
	}
	if start.Line == end.Line {
		// Merging always shortens the line, so no width is needed.
		return true
	}
	if linelen.Config.Len <= 0 {
		// A join across lines moves text onto the first line. Without
		// a width there is nothing to say the result fits.
		return false
	}
	first, last := lines[start.Line-1], lines[end.Line-1]
	if start.Column-1 > len(first) || end.Column-1 > len(last) {
		return false
	}
	joined := first[:start.Column-1] + merged + last[end.Column-1:]
	return linelen.CheckLine([]byte(joined))
}
