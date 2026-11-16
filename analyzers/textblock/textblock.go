// Package textblock provides an analysis.Analyzer that reports a block of text
// built one line at a time from string constants.
//
// Two shapes build the same block: a strings.Join of two or more string
// literals with a separator of one newline, and a chain of four or more string
// literal operands where every operand but the last ends with a newline. The
// chain is reported once, at its outermost +. One raw string literal holds
// the block. The spec says a raw string literal "may contain any character
// except the back quote"
// (https://go.dev/ref/spec#String_literals).
//
// A block with a back quote in any line stays as it is, because no raw
// literal can hold it. A carriage return is dropped from a raw literal's
// value, and a NUL or invalid UTF-8 byte is not source text, so a block
// holding one of those stays as it is too. A keyed slice literal is left
// alone, because the keys decide the order. A named constant element or
// operand is left alone in either shape, because the line is written
// elsewhere. A + chain whose first operand is only newlines and whose last
// operand does not end with a newline joins a newline to a suffix; it is a
// fragment rather than a block. strconcat leaves a newline Join of string
// literals that a raw literal can hold to this rule.
//
// Report only. The rewrite preserves the value, but it moves every line to
// the source's margin, and a line whose leading whitespace is part of the
// generated text is a case the author has to check. A comment between the
// parts is lost, because a raw literal cannot hold it. The README catalogs
// every check, exemption, and limit.
package textblock

import (
	"go/ast"
	"go/constant"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/rawstring"
)

var Analyzer = &analysis.Analyzer{
	Name: "textblock",
	Doc:  "reports a text block built a line at a time from string constants",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Walk(&walker{pass: pass}, file)
	}
	return nil, nil
}

// walker visits a file with the parent node at hand, so a chain is reported
// once, at its outermost +.
type walker struct {
	pass  *analysis.Pass
	stack []ast.Node
}

func (w *walker) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		w.stack = w.stack[:len(w.stack)-1]
		return nil
	}
	switch n := n.(type) {
	case *ast.CallExpr:
		if checkJoin(w.pass, n) {
			return nil
		}
	case *ast.BinaryExpr:
		if additiveParent(w.stack) {
			break
		}
		if checkConcat(w.pass, n) {
			return nil
		}
	}
	w.stack = append(w.stack, n)
	return w
}

// additiveParent reports whether the nearest non-parenthesis ancestor of the
// current node is a + expression.
func additiveParent(stack []ast.Node) bool {
	for _, s := range slices.Backward(stack) {
		switch p := s.(type) {
		case *ast.ParenExpr:
			continue
		case *ast.BinaryExpr:
			return p.Op == token.ADD
		default:
			return false
		}
	}
	return false
}

// checkJoin reports a strings.Join call over a composite literal of string
// literals with a newline separator, and returns whether the call was handled.
func checkJoin(pass *analysis.Pass, call *ast.CallExpr) bool {
	if !isJoin(pass, call) {
		return false
	}
	lit, ok := ast.Unparen(call.Args[0]).(*ast.CompositeLit)
	if !ok || len(lit.Elts) < 2 {
		return false
	}
	value, ok := joinValue(lit)
	if !ok || !rawstring.Holds(value) {
		return false
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "text block built a line at a time; use a raw string literal",
	})
	return true
}

// isJoin reports whether call selects strings.Join and separates its parts
// with one newline.
func isJoin(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 2 {
		return false
	}
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.FullName() != "strings.Join" {
		return false
	}
	sep, ok := constantValue(pass, call.Args[1])
	return ok && sep == "\n"
}

// joinValue returns the text a Join call over lit writes, and whether every
// element of lit is a string literal.
func joinValue(lit *ast.CompositeLit) (string, bool) {
	parts := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		lit, ok := ast.Unparen(elt).(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return "", false
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n"), true
}

// checkConcat reports a chain of four or more string literals joined by +
// where every operand but the last ends with a newline, and returns whether
// the chain was handled.
func checkConcat(pass *analysis.Pass, bin *ast.BinaryExpr) bool {
	if bin.Op != token.ADD {
		return false
	}
	operands, ok := flatten(bin)
	if !ok || len(operands) < 4 {
		return false
	}
	firstIsNewlines := strings.TrimRight(operands[0], "\n") == ""
	last := operands[len(operands)-1]
	if firstIsNewlines && !strings.HasSuffix(last, "\n") {
		return false
	}
	var value strings.Builder
	for i, operand := range operands {
		if i < len(operands)-1 && !strings.HasSuffix(operand, "\n") {
			return false
		}
		value.WriteString(operand)
	}
	if !rawstring.Holds(value.String()) {
		return false
	}
	pass.Report(analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: "text block built a line at a time; use a raw string literal",
	})
	return true
}

// flatten returns the string literal operands of a chain of + expressions, and
// whether every operand is a string literal.
func flatten(bin *ast.BinaryExpr) ([]string, bool) {
	var (
		out  []string
		walk func(ast.Expr) bool
	)
	walk = func(e ast.Expr) bool {
		if b, ok := ast.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.ADD {
			return walk(b.X) && walk(b.Y)
		}
		lit, ok := ast.Unparen(e).(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return false
		}
		out = append(out, value)
		return true
	}
	if !walk(bin) {
		return nil, false
	}
	return out, true
}

// constantValue returns e's value as a string, and whether e is a constant
// string.
func constantValue(pass *analysis.Pass, e ast.Expr) (string, bool) {
	v := pass.TypesInfo.Types[e].Value
	if v == nil || v.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(v), true
}
