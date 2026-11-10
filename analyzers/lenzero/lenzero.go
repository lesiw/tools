// Package lenzero provides an analysis.Analyzer that reports a comparison
// of len with zero that is always true or always false.
//
// len returns a non-negative int. len(x) < 0 is always false, and len(x) >= 0
// is always true. Neither comparison checks anything.
package lenzero

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "lenzero",
	Doc:  "reports a comparison of len with zero that is always the same",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || !constant(pass, bin) {
				return true
			}
			answer := strconv.FormatBool(truth(bin))
			pass.Report(analysis.Diagnostic{
				Pos:     bin.Pos(),
				End:     bin.End(),
				Message: "len is never negative; this is always " + answer,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Use the constant",
					TextEdits: []analysis.TextEdit{{
						Pos:     bin.Pos(),
						End:     bin.End(),
						NewText: []byte(answer),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// truth returns the answer the comparison always has.
func truth(e *ast.BinaryExpr) bool {
	return e.Op == token.GEQ || e.Op == token.LEQ
}

// constant reports whether e compares len with zero in a way whose answer
// cannot change.
func constant(pass *analysis.Pass, e *ast.BinaryExpr) bool {
	left, right := isLen(pass, e.X), isLen(pass, e.Y)
	switch {
	case left && isZero(e.Y):
		return e.Op == token.LSS || e.Op == token.GEQ
	case right && isZero(e.X):
		return e.Op == token.GTR || e.Op == token.LEQ
	}
	return false
}

// isLen reports whether e is a call to the builtin len.
func isLen(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("len")
}

// isZero reports whether e is the literal 0.
func isZero(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}
