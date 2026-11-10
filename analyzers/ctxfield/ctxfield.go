// Package ctxfield provides an analysis.Analyzer that reports a
// context.Context in a struct field.
//
// A context has the credentials, the deadline and the cancellation of one
// request. A struct that stores one cannot serve another request, and the
// field makes a dependency implicit that the methods could take as a
// parameter.
package ctxfield

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ctxfield",
	Doc:  "reports a context.Context in a field",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			structType, ok := n.(*ast.StructType)
			if !ok || structType.Fields == nil {
				return true
			}
			for _, field := range structType.Fields.List {
				// An embedded context is how a type implements
				// the interface, not a stored deadline.
				if len(field.Names) == 0 {
					continue
				}
				if isContext(pass, field.Type) {
					pass.Reportf(field.Pos(), "context.Context in a field")
				}
			}
			return true
		})
	}
	return nil, nil
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
