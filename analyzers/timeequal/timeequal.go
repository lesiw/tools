// Package timeequal provides an analysis.Analyzer that reports a time.Time
// compared with == or !=.
//
// The time package says what the operator does: "Note that the Go == operator
// compares not just the time instant but also the Location and the monotonic
// clock reading." The Time doc adds: "In general, prefer t.Equal(u) to t == u,
// since t.Equal uses the most accurate comparison available and correctly
// handles the case when only one of its arguments has a monotonic clock
// reading."
//
// A comparison against the zero value is a check for an unset time: "the
// Time.IsZero method gives a simple way of detecting a time that has not been
// initialized explicitly". IsZero reads the instant alone, so a zero instant
// in a named zone counts as unset where == calls it set.
//
// The fix writes Equal, or IsZero for a comparison against the zero value,
// and is withheld where the edit would lose a comment, join lines, or take the
// line past the width. Only two time.Time values are examined: a case in a
// switch on one, a map keyed by one, and a comparison of a struct or a defined
// type over one are not.
//
// The rule is like revive's time-equal, which reports the comparison without a
// fix and without a zero-value case.
package timeequal

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "timeequal",
	Doc:  "reports a time.Time compared with == or !=",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		checkFile(pass, f)
	}
	return nil, nil
}

func checkFile(pass *analysis.Pass, f *ast.File) {
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || bin.Op != token.EQL && bin.Op != token.NEQ {
			return true
		}
		if !isTime(pass, bin.X) || !isTime(pass, bin.Y) {
			return true
		}
		span, negate := target(stack, bin)
		report(pass, f, bin, span, negate)
		return true
	})
}

// target returns the node a fix replaces and whether a ! covers it. A
// parenthesis or a ! is taken with the comparison because the replacement
// binds the same way.
func target(stack []ast.Node, bin *ast.BinaryExpr) (ast.Expr, bool) {
	var (
		span   ast.Expr = bin
		negate bool
	)
	for i := len(stack) - 2; i >= 0; i-- {
		switch parent := stack[i].(type) {
		case *ast.ParenExpr:
			span = parent
		case *ast.UnaryExpr:
			if parent.Op != token.NOT {
				return span, negate
			}
			span, negate = parent, !negate
		default:
			return span, negate
		}
	}
	return span, negate
}

func report(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, span ast.Expr, negate bool) {
	var (
		from, to     = span.Pos(), span.End()
		xZero, yZero = zeroLiteral(bin.X), zeroLiteral(bin.Y)
	)
	if xZero && yZero {
		// Both operands are the literal, so neither is the time under test.
		return
	}
	recv, arg := bin.X, bin.Y
	if xZero {
		recv, arg = arg, recv
	}
	zero := xZero || yZero
	message := bin.Op.String() + " on a time.Time compares more than the instant"
	fix := "Compare with Equal"
	if zero {
		message = "time.Time compared with the zero value"
		fix = "Compare with IsZero"
	}
	diag := analysis.Diagnostic{
		Pos:     from,
		End:     to,
		Message: message,
	}
	not := bin.Op == token.NEQ
	if negate {
		not = !not
	}
	text, err := rewrite(pass.Fset, recv, arg, zero, not)
	if err != nil || strings.Contains(text, "\n") {
		// Nothing is quoted that a reader cannot write on one line.
		pass.Report(diag)
		return
	}
	diag.Message += "; write " + text
	if fits(pass, from, to, text) && !commented(file, from, to) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: fix,
			TextEdits: []analysis.TextEdit{{
				Pos:     from,
				End:     to,
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
}

// rewrite returns the text that compares recv with arg, negated when not is
// set. A comparison against the zero value names IsZero.
func rewrite(fset *token.FileSet, recv, arg ast.Expr, zero, not bool) (string, error) {
	var bang string
	if not {
		bang = "!"
	}
	text, err := receiverText(fset, recv)
	if err != nil {
		return "", err
	}
	if zero {
		return bang + text + ".IsZero()", nil
	}
	call, err := render(fset, ast.Unparen(arg))
	if err != nil {
		return "", err
	}
	return bang + text + ".Equal(" + call + ")", nil
}

// receiverText returns the text of e where a selector follows it.
func receiverText(fset *token.FileSet, e ast.Expr) (string, error) {
	e = ast.Unparen(e)
	text, err := render(fset, e)
	if err != nil {
		return "", err
	}
	switch e.(type) {
	case *ast.StarExpr, *ast.UnaryExpr:
		// A selector after either binds to its operand: *p.Equal(q) is
		// *(p.Equal(q)).
		return "(" + text + ")", nil
	}
	return text, nil
}

// render returns the source of one expression.
func render(fset *token.FileSet, e ast.Expr) (string, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, e); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// isTime reports whether e is a time.Time.
func isTime(pass *analysis.Pass, e ast.Expr) bool {
	named, ok := types.Unalias(pass.TypesInfo.TypeOf(e)).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "time" &&
		obj.Name() == "Time"
}

// zeroLiteral reports whether e is the zero time.Time literal. The caller has
// already checked that e is a time.Time, so no type is looked up here.
func zeroLiteral(e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	return ok && len(lit.Elts) == 0
}

// commented reports whether a comment lies in the text a fix would replace.
func commented(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if c.Pos() >= from && c.Pos() < to {
				return true
			}
		}
	}
	return false
}

// fits reports whether text may replace the range from to. The range lies on
// one line, and the rewritten line is within the width unless the line is
// already past it.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	// Written positions, which a //line directive does not move.
	start, end := pass.Fset.PositionFor(from, false),
		pass.Fset.PositionFor(to, false)
	if start.Line != end.Line {
		return false
	}
	if linelen.Config.Len <= 0 {
		return true
	}
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
	line := lines[start.Line-1]
	if start.Column > len(line)+1 || end.Column > len(line)+1 {
		return false
	}
	if !linelen.CheckLine(line) {
		return true
	}
	var rewritten bytes.Buffer
	rewritten.Write(line[:start.Column-1])
	rewritten.WriteString(text)
	rewritten.Write(line[end.Column-1:])
	return linelen.CheckLine(rewritten.Bytes())
}
