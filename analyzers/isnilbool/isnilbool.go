// Package isnilbool provides an analysis.Analyzer that reports a comparison of
// a comparison with a boolean literal.
//
// The specification says a comparison yields an untyped boolean value
// (https://go.dev/ref/spec#Comparison_operators). A comparison of a comparison
// with true using ==, or with false using !=, has the value of the inner
// comparison, so the inner comparison can stand in its place. The literal may
// be on either side, and either comparison may be parenthesized.
//
// The fix writes the inner comparison in place of the outer one. A comment in
// the deleted text withholds the fix. == false and != true would need a
// negation, so they are not reported.
//
// The rule is adapted from staticcheck's S1002, which reports a comparison
// with a boolean constant for any boolean operand.
package isnilbool

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "isnilbool",
	Doc:  "reports a comparison of a comparison with a boolean literal",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			outer, ok := n.(*ast.BinaryExpr)
			if ok && check(pass, file, outer) {
				return false
			}
			return true
		})
	}
	return nil, nil
}

// check reports outer when it compares a comparison with true or false and the
// comparison is already the value. It reports whether it found one.
func check(pass *analysis.Pass, file *ast.File, outer *ast.BinaryExpr) bool {
	if outer.Op != token.EQL && outer.Op != token.NEQ {
		return false
	}
	operand, literal, ok := sides(pass, outer)
	if !ok || !comparison(operand) {
		return false
	}
	// == true and != false leave the operand as it is.
	if (outer.Op == token.EQL) != literal {
		return false
	}
	message := "comparison with true; write the inner comparison"
	if !literal {
		message = "comparison with false; write the inner comparison"
	}
	d := analysis.Diagnostic{
		Pos:     outer.Pos(),
		End:     outer.End(),
		Message: message,
	}
	pos, end := deleted(outer, operand)
	if !commented(file.Comments, pos, end) {
		if src, ok := source(pass, operand); ok {
			d.SuggestedFixes = []analysis.SuggestedFix{{
				Message: "Write the inner comparison",
				TextEdits: []analysis.TextEdit{{
					Pos:     outer.Pos(),
					End:     outer.End(),
					NewText: src,
				}},
			}}
		}
	}
	pass.Report(d)
	return true
}

// sides returns the expression on the side of the literal and the literal's
// value. It reports false when neither side is the predeclared literal.
func sides(pass *analysis.Pass, bin *ast.BinaryExpr) (operand ast.Expr, literal, ok bool) {
	if value, is := truth(pass, bin.Y); is {
		return bin.X, value, true
	}
	if value, is := truth(pass, bin.X); is {
		return bin.Y, value, true
	}
	return nil, false, false
}

// truth returns the value of the predeclared boolean constant e names, and
// whether e names one. A name that shadows the constant is not one.
func truth(pass *analysis.Pass, e ast.Expr) (value, ok bool) {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false, false
	}
	switch pass.TypesInfo.Uses[id] {
	case types.Universe.Lookup("true"):
		return true, true
	case types.Universe.Lookup("false"):
		return false, true
	}
	return false, false
}

// comparison reports whether e is a comparison.
func comparison(e ast.Expr) bool {
	bin, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok {
		return false
	}
	switch bin.Op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	}
	return false
}

// deleted returns the span the fix removes: the literal, the operator, and the
// space between them.
func deleted(bin *ast.BinaryExpr, operand ast.Expr) (pos, end token.Pos) {
	if operand == bin.X {
		return operand.End(), bin.End()
	}
	return bin.Pos(), operand.Pos()
}

// source returns the source text of e.
func source(pass *analysis.Pass, e ast.Expr) ([]byte, bool) {
	file := pass.Fset.File(e.Pos())
	if file == nil {
		return nil, false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return nil, false
	}
	lo, hi := file.Offset(e.Pos()), file.Offset(e.End())
	if lo < 0 || hi < lo || hi > len(src) {
		return nil, false
	}
	return bytes.Clone(src[lo:hi]), true
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
