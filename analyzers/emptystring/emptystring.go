// Package emptystring provides an analysis.Analyzer that reports a test of
// whether a string is empty, written as a comparison of its length with zero.
//
// The spec's String types section says the length of a string is never
// negative. So len(s) == 0 and len(s) <= 0 both become s == "", and
// len(s) != 0 and len(s) > 0 both become s != ""; a comparison with "" is
// the form a reader expects. len(s) < 0 is always false and len(s) >= 0 is
// always true, and lenzero reports such a comparison as the constant it is.
//
// The operand's type must be a string, or a type whose underlying type is a
// string. A type parameter is not examined.
//
// The fix is withheld when a comment is inside the comparison.
//
// The rule is inspired by go-critic's emptyStringTest. That rule matches the
// same four comparisons with the length on the left; this one matches the zero
// on either side.
package emptystring

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptystring",
	Doc:  "reports a test of whether a string is empty, written as a comparison of its length with zero",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			if str, op, ok := test(pass, bin); ok {
				report(pass, file, bin, str, op)
			}
			return true
		})
	}
	return nil, nil
}

// test returns the string operand and the operator to write between it and ""
// when bin is a comparison of a string's length with zero.
func test(pass *analysis.Pass, bin *ast.BinaryExpr) (ast.Expr, token.Token, bool) {
	var (
		str, ok = stringOperand(pass, bin.X, bin.Y)
		op      = bin.Op
	)
	if !ok {
		str, ok = stringOperand(pass, bin.Y, bin.X)
		op = flip(op)
	}
	if !ok {
		return nil, 0, false
	}
	switch op {
	case token.EQL, token.LEQ:
		return str, token.EQL, true
	case token.NEQ, token.GTR:
		return str, token.NEQ, true
	}
	return nil, 0, false
}

// stringOperand returns the string when call is len of a string and other is
// the literal 0.
func stringOperand(pass *analysis.Pass, call, other ast.Expr) (ast.Expr, bool) {
	call, other = ast.Unparen(call), ast.Unparen(other)
	lit, ok := other.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT || lit.Value != "0" {
		return nil, false
	}
	expr, ok := call.(*ast.CallExpr)
	if !ok || len(expr.Args) != 1 {
		return nil, false
	}
	id, ok := expr.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("len") {
		return nil, false
	}
	typ := pass.TypesInfo.TypeOf(expr.Args[0])
	if typ == nil {
		return nil, false
	}
	basic, ok := typ.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsString == 0 {
		return nil, false
	}
	return expr.Args[0], true
}

// flip returns the operator that reads the same with its operands swapped.
func flip(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.GTR:
		return token.LSS
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	}
	return op
}

// report reports bin and, unless a comment inside it would be lost, suggests
// the comparison with "" in its place.
func report(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, str ast.Expr, op token.Token) {
	var (
		from, to = bin.Pos(), bin.End()
		diag     = analysis.Diagnostic{
			Pos:     from,
			End:     to,
			Message: `compare the string with "" instead`,
		}
	)
	if !commented(file, from, to) {
		text := types.ExprString(str) + " " + op.String() + ` ""`
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Compare the string",
			TextEdits: []analysis.TextEdit{{
				Pos:     from,
				End:     to,
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
