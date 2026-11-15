// Package uses holds the facts analyzers share about a package's functions:
// which function a call names, which functions are held as values, which
// methods its interfaces declare, and whether two signatures have the same
// parameters and results.
package uses

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// CalleeOf returns the identifier and the function that a call's Fun names. It
// looks through parentheses and a type instantiation, and finds nothing in a
// value, a field, or a conversion.
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

// Declarations holds the unexported methods that the interfaces in a package
// declare, their own and those they embed from that package, keyed by name.
type Declarations map[string][]*types.Func

// InterfaceMethods returns the methods that the interfaces in the package
// declare. A method declared here cannot satisfy an unexported name from
// another package, so those are not counted.
func InterfaceMethods(pass *analysis.Pass) Declarations {
	out := make(Declarations)
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
					out[method.Name()] = append(out[method.Name()], method)
				}
			}
			return true
		})
	}
	return out
}

// Has reports whether an interface in the package declares fn's name with fn's
// signature, which is what a method written to match it has. Only a method can
// satisfy an interface, so a function is never declared by one.
func (d Declarations) Has(fn *types.Func) bool {
	sig := fn.Signature()
	if sig.Recv() == nil {
		return false
	}
	for _, method := range d[fn.Name()] {
		methodSig := method.Signature()
		if SameSignature(methodSig, sig) {
			return true
		}
		// Type parameters never compare identical across declarations, so a
		// signature that mentions one is compared by shape instead.
		if shaped(sig) || shaped(methodSig) {
			if shapeOf(methodSig) == shapeOf(sig) {
				return true
			}
		}
	}
	return false
}

// shaped reports whether sig has to be compared by shape, because a type
// parameter takes the place of a type somewhere in it.
func shaped(sig *types.Signature) bool {
	if sig.TypeParams() != nil || sig.RecvTypeParams() != nil {
		return true
	}
	for v := range sig.Params().Variables() {
		if parameterized(v.Type()) {
			return true
		}
	}
	for v := range sig.Results().Variables() {
		if parameterized(v.Type()) {
			return true
		}
	}
	return false
}

// parameterized reports whether t is a type parameter or holds one.
func parameterized(t types.Type) bool {
	switch t := t.(type) {
	case *types.TypeParam:
		return true
	case *types.Slice:
		return parameterized(t.Elem())
	case *types.Array:
		return parameterized(t.Elem())
	case *types.Pointer:
		return parameterized(t.Elem())
	case *types.Chan:
		return parameterized(t.Elem())
	case *types.Map:
		return parameterized(t.Key()) || parameterized(t.Elem())
	}
	return false
}

// shape is the part of a signature that type parameters do not change.
type shape struct {
	params   int
	results  int
	variadic bool
}

// shapeOf returns the shape of sig.
func shapeOf(sig *types.Signature) shape {
	return shape{
		params:   sig.Params().Len(),
		results:  sig.Results().Len(),
		variadic: sig.Variadic(),
	}
}

// SameSignature reports whether x and y take and return the same types. A
// receiver is not part of the comparison, and neither are parameter names.
//
// The variadic flag is compared on its own because a parameter tuple holds
// ...int and []int as the same type, so a slice-taking function and a variadic
// one would otherwise compare equal and any difference between them lost.
func SameSignature(x, y *types.Signature) bool {
	return x.Variadic() == y.Variadic() &&
		types.Identical(x.Params(), y.Params()) &&
		types.Identical(x.Results(), y.Results())
}
