// Package ctxvaluetype provides an analysis.Analyzer that reports a
// context.WithValue call whose value argument is the nil literal.
//
// Storing nil changes what Value returns only when the parent already holds a
// non-nil value under the key. The rule reports the call regardless of the
// parent.
//
// No fix is offered: replacing the call with its parent could change what
// Value returns where the parent holds a non-nil value.
//
// The README lists the forms that are left alone.
package ctxvaluetype

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "ctxvaluetype",
	Doc:  "reports a context.WithValue call whose value argument is the nil literal",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 || !isWithValue(pass, call) {
				return true
			}
			if pass.TypesInfo.Types[call.Args[2]].IsNil() {
				pass.Reportf(
					call.Args[2].Pos(),
					"context.WithValue is called with the nil literal as its value",
				)
			}
			return true
		})
	}
	return nil, nil
}

// isWithValue reports whether call names context.WithValue.
func isWithValue(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "context" &&
		fn.Name() == "WithValue"
}
