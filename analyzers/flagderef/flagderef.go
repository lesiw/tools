// Package flagderef provides an analysis.Analyzer that reports a flag value
// read before Parse can set it.
//
// flag.Bool and its siblings register a flag and return a pointer to its
// value. Before Parse, the pointer points at the default value. The Var form
// takes the address of a variable that Parse fills in.
package flagderef

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "flagderef",
	Doc:  "reports a flag value read before Parse can set it",
	Run:  run,
}

// constructors are the flag functions that return a pointer to a value.
var constructors = map[string]struct{}{
	"Bool":     {},
	"Duration": {},
	"Float64":  {},
	"Func":     {},
	"Int":      {},
	"Int64":    {},
	"String":   {},
	"Uint":     {},
	"Uint64":   {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			star, ok := n.(*ast.StarExpr)
			if !ok || !isConstructor(pass, star.X) {
				return true
			}
			pass.Reportf(star.Pos(), "flag read before Parse")
			return true
		})
	}
	return nil, nil
}

// isConstructor reports whether e is a call to a flag constructor.
func isConstructor(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if _, known := constructors[sel.Sel.Name]; !known {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == "flag"
}
