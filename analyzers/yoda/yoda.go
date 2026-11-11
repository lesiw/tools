// Package yoda provides an analysis.Analyzer that reports a comparison with
// the constant on the left.
//
// Go code reads left to right as a sentence about the variable: "x is nil",
// not "nil is x". The constant-first form comes from C, where it guards
// against a mistyped assignment, and that mistake is a compile error in Go.
package yoda

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "yoda",
	Doc:  "reports a comparison with the constant on the left",
	Run:  run,
}

// comparisons are the operators whose sides can be swapped without changing
// the answer.
var comparisons = map[token.Token]struct{}{
	token.EQL: {},
	token.NEQ: {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			if _, known := comparisons[bin.Op]; !known {
				return true
			}
			if !constant(bin.X) || constant(bin.Y) {
				return true
			}
			var (
				left  = types.ExprString(bin.Y)
				right = types.ExprString(bin.X)
				op    = " " + bin.Op.String() + " "
			)
			pass.Report(analysis.Diagnostic{
				Pos:     bin.Pos(),
				End:     bin.End(),
				Message: "constant on the left of a comparison",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Put the value first",
					TextEdits: []analysis.TextEdit{{
						Pos:     bin.Pos(),
						End:     bin.End(),
						NewText: []byte(left + op + right),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// constant reports whether e is a literal or nil.
func constant(e ast.Expr) bool {
	if _, ok := e.(*ast.BasicLit); ok {
		return true
	}
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil" && id.Obj == nil
}
