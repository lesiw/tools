// Package emptystring provides an analysis.Analyzer that reports a string
// compared with its own length.
//
// len(s) == 0 and s == "" are the same test, and the second is the form a
// reader expects. Like go-critic's emptyStringTest.
package emptystring

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptystring",
	Doc:  "reports a string compared with its own length",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || !comparison(bin.Op) {
				return true
			}
			if str, ok := stringOperand(pass, bin.X, bin.Y); ok {
				report(pass, bin, str, bin.Op)
			} else if str, ok := stringOperand(pass, bin.Y, bin.X); ok {
				report(pass, bin, str, flip(bin.Op))
			}
			return true
		})
	}
	return nil, nil
}

// comparison reports whether op compares against zero in a way a string
// comparison can express.
func comparison(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
		return true
	}
	return false
}

// stringOperand returns the string when call is len of a string and other is
// the literal 0.
func stringOperand(pass *analysis.Pass, call, other ast.Expr) (ast.Expr, bool) {
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
	basic, ok := types.Unalias(typ).(*types.Basic)
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

// report suggests the comparison that names the string.
func report(pass *analysis.Pass, bin *ast.BinaryExpr, str ast.Expr, op token.Token) {
	text := types.ExprString(str) + " " + op.String() + ` ""`
	pass.Report(analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: `compare the string with "" instead`,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Compare the string",
			TextEdits: []analysis.TextEdit{{
				Pos:     bin.Pos(),
				End:     bin.End(),
				NewText: []byte(text),
			}},
		}},
	})
}
