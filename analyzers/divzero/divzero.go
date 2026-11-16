// Package divzero provides an analysis.Analyzer that reports integer division
// of two integer literals whose quotient is zero.
//
// The spec's Arithmetic operators section says the quotient x / y is
// "truncated towards zero" (https://go.dev/ref/spec#Arithmetic_operators), so
// 1 / 2 is 0. The rule reports a division of two integer literals, with a
// unary sign or parentheses allowed, and leaves named constants alone because
// a division of named values is often deliberate.
//
// The rule is adapted from staticcheck's SA4025, which reports the same
// divisions and offers no fix. The quotient is zero in every context, so
// writing 0 in place of the division keeps the program the same. The fix is
// withheld where a comment sits inside the division, because the edit would
// delete the comment. The README catalogs every check, exemption, and fix.
//
//	func half() int {
//		return 1 / 2
//	}
package divzero

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "divzero",
	Doc:  "reports integer division of literals whose quotient is zero",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || bin.Op != token.QUO {
				return true
			}
			if !integerLiteral(bin.X) || !integerLiteral(bin.Y) {
				return true
			}
			value := pass.TypesInfo.Types[bin].Value
			if value == nil || constant.Sign(value) != 0 {
				return true
			}
			expr := types.ExprString(bin)
			d := analysis.Diagnostic{
				Pos:     bin.Pos(),
				End:     bin.End(),
				Message: "integer division " + expr + " results in zero",
			}
			if !commented(file.Comments, bin.Pos(), bin.End()) {
				d.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Replace the division with 0",
					TextEdits: []analysis.TextEdit{{
						Pos:     bin.Pos(),
						End:     bin.End(),
						NewText: []byte("0"),
					}},
				}}
			}
			pass.Report(d)
			return true
		})
	}
	return nil, nil
}

// integerLiteral reports whether e is an integer literal, with parentheses
// and a unary sign allowed.
func integerLiteral(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.BasicLit:
		return e.Kind == token.INT
	case *ast.UnaryExpr:
		if e.Op != token.ADD && e.Op != token.SUB {
			return false
		}
		return integerLiteral(e.X)
	}
	return false
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
