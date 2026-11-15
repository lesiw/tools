// Package uses finds the functions a package calls or holds, and the methods
// its interfaces declare.
package uses

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// CalleeOf returns the identifier that names the function expr names, and the
// function itself. It looks through parentheses and a type instantiation, and
// finds nothing in a value, a field, or a conversion.
func CalleeOf(pass *analysis.Pass, expr ast.Expr) (*ast.Ident, *types.Func, bool) {
	for {
		switch x := ast.Unparen(expr).(type) {
		case *ast.IndexExpr:
			if !pass.TypesInfo.Types[x.Index].IsType() {
				return nil, nil, false
			}
			expr = x.X
		case *ast.IndexListExpr:
			expr = x.X
		case *ast.Ident:
			fn, ok := pass.TypesInfo.Uses[x].(*types.Func)
			return x, fn, ok
		case *ast.SelectorExpr:
			fn, ok := pass.TypesInfo.Uses[x.Sel].(*types.Func)
			return x.Sel, fn, ok
		default:
			return nil, nil, false
		}
	}
}

// Held returns the functions the package uses as a value rather than calls.
func Held(pass *analysis.Pass) map[*types.Func]struct{} {
	called := make(map[*ast.Ident]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, _, ok := CalleeOf(pass, call.Fun); ok {
				called[name] = struct{}{}
			}
			return true
		})
	}
	out := make(map[*types.Func]struct{})
	for id, obj := range pass.TypesInfo.Uses {
		if _, ok := called[id]; ok {
			continue
		}
		if fn, ok := obj.(*types.Func); ok {
			out[fn] = struct{}{}
		}
	}
	return out
}

// InterfaceMethods returns the names of the unexported methods that the
// interfaces in the package declare. A method declared here cannot satisfy an
// unexported name from another package, so those are not counted.
func InterfaceMethods(pass *analysis.Pass) map[string]struct{} {
	out := make(map[string]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			expr, ok := n.(*ast.InterfaceType)
			if !ok {
				return true
			}
			iface, ok := pass.TypesInfo.TypeOf(expr).(*types.Interface)
			if !ok {
				return true
			}
			for method := range iface.Methods() {
				if !method.Exported() && method.Pkg() == pass.Pkg {
					out[method.Name()] = struct{}{}
				}
			}
			return true
		})
	}
	return out
}
