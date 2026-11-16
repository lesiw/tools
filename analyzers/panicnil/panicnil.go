// Package panicnil provides an analysis.Analyzer that reports a call to panic
// with the predeclared nil.
//
// The call passes nil, so the crash report can say only that the argument was
// nil.
//
// The builtin package documents the behavior: "Starting in Go 1.21, calling
// panic with a nil interface value or an untyped nil causes a run-time error
// (a different panic). The GODEBUG setting panicnil=1 disables the run-time
// error." (https://pkg.go.dev/builtin#panic). A module that declares Go 1.20
// or earlier keeps the older behavior, where recover returns nil
// (https://pkg.go.dev/runtime#PanicNilError); a deferred recover returns nil
// under runtime.Goexit as well (https://pkg.go.dev/runtime#Goexit). Both
// behaviors leave the report with nothing from the author.
//
// Report only: the repair is a message that states why the program stopped,
// which the analyzer cannot supply. The README lists the exemptions.
package panicnil

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "panicnil",
	Doc:  "reports a call to panic with the predeclared nil",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && isNilPanic(pass, call) {
				pass.Report(analysis.Diagnostic{
					Pos:     call.Pos(),
					End:     call.End(),
					Message: "a nil panic gives no reason for stopping",
				})
			}
			return true
		})
	}
	return nil, nil
}

// isNilPanic reports whether call is a call to the builtin panic that passes
// the predeclared nil.
func isNilPanic(pass *analysis.Pass, call *ast.CallExpr) bool {
	return builtin(pass, call.Fun, "panic") &&
		builtin(pass, call.Args[0], "nil")
}

// builtin reports whether e names the predeclared name. The type checker
// records the object each identifier denotes, so a shadowed name is a
// different object.
func builtin(pass *analysis.Pass, e ast.Expr, name string) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup(name)
}
