// Package offby1 provides an analysis.Analyzer that reports an index that is
// always out of range.
//
// len(x) is the number of elements, so the last one is at len(x)-1. Indexing
// at len(x) panics on every call. The compiler accepts it. Like go-critic's
// offBy1.
package offby1

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "offby1",
	Doc:  "reports an index that is always out of range",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.IndexExpr:
				if pastEnd(pass, n) {
					// A fix would guess which index was meant, and
					// the last one is only sometimes right.
					pass.Reportf(n.Pos(), "index out of range")
				}
			}
			return true
		})
	}
	return nil, nil
}

// pastEnd reports whether e is x[len(x)].
func pastEnd(pass *analysis.Pass, e *ast.IndexExpr) bool {
	slice, ok := e.X.(*ast.Ident)
	if !ok {
		return false
	}
	call, ok := e.Index.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("len") {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(arg) == pass.TypesInfo.ObjectOf(slice)
}
