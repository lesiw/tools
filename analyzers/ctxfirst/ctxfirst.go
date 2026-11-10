// Package ctxfirst provides an analysis.Analyzer that reports a
// context.Context parameter that is not the first one.
//
// A context has the credentials, the deadline and the cancellation for one
// request. Every function that takes one passes it along the same chain.
// Keeping it first means a reader finds it in the same place everywhere.
package ctxfirst

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ctxfirst",
	Doc:  "reports a context parameter that is not the first one",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			decl, ok := n.(*ast.FuncDecl)
			if !ok || decl.Type.Params == nil {
				return true
			}
			check(pass, decl.Type.Params)
			return true
		})
	}
	return nil, nil
}

// check reports the first context parameter when it is not in first position.
func check(pass *analysis.Pass, params *ast.FieldList) {
	for i, field := range params.List {
		if !isContext(pass, field.Type) {
			continue
		}
		if i > 0 {
			pass.Reportf(field.Type.Pos(), "context.Context is not the first parameter")
		}
		return
	}
}

// isContext reports whether e is a context.Context.
func isContext(pass *analysis.Pass, e ast.Expr) bool {
	named, ok := types.Unalias(pass.TypesInfo.TypeOf(e)).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" &&
		obj.Name() == "Context"
}
