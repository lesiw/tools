// Package newderef provides an analysis.Analyzer that reports a dereference of
// a fresh allocation.
//
// *new(T) allocates a T, zeroes it, and reads it back, which is T{}. The
// expression suggests a value the program already had. A type parameter is
// exempt: there is no composite literal for one. Like go-critic's newDeref.
package newderef

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "newderef",
	Doc:  "reports a dereference of a fresh allocation",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			star, ok := n.(*ast.StarExpr)
			if !ok || !fresh(pass, star.X) {
				return true
			}
			pass.Reportf(star.Pos(), "dereferenced allocation; the zero value will do")
			return true
		})
	}
	return nil, nil
}

// fresh reports whether e is new(T) for a T that is not a type parameter.
func fresh(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("new") {
		return false
	}
	typ := pass.TypesInfo.TypeOf(call.Args[0])
	if typ == nil {
		return false
	}
	_, isParam := typ.(*types.TypeParam)
	return !isParam
}
