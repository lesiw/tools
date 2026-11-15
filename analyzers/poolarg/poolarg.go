// Package poolarg provides an analysis.Analyzer that reports a value stored in
// a sync.Pool that is not one pointer word.
//
// "Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector." The compiler stores a value
// directly in an interface's data word only when the value is one word that a
// collector reads as a pointer (cmd/compile/internal/types.IsDirectIface: "Can
// this type be stored directly in an interface word?"). For any other value
// whose type is not an interface, the data word points at memory holding it: a
// copy of a slice header, a string, or a struct, or static data the compiler
// already holds.
//
// The rule reports without a fix, because storing a pointer changes the type
// the pool holds and the code that reads the pool asserts that type.
//
// The rule is adapted from staticcheck's SA6002, which reports a value whose
// type is not pointer-like, and a slice even though it is. This rule compares
// the size of the type, so it reports fewer values.
//
// The README catalogs every check and exemption.
package poolarg

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "poolarg",
	Doc:  "reports a value stored in a sync.Pool that is not one pointer word",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isPoolPut(pass, call.Fun) {
				return true
			}
			// A method expression puts the receiver first.
			arg := call.Args[len(call.Args)-1]
			typ := pass.TypesInfo.TypeOf(arg)
			if !constant(pass, arg) && indirect(typ, pass.TypesSizes) {
				pass.Report(analysis.Diagnostic{
					Pos:     arg.Pos(),
					End:     arg.End(),
					Message: "value stored in the pool is not a pointer; store a pointer",
				})
			}
			return true
		})
	}
	return nil, nil
}

// isPoolPut reports whether fun is (*sync.Pool).Put, selected on a pool, on an
// embedded one, or on the type.
func isPoolPut(pass *analysis.Pass, fun ast.Expr) bool {
	sel, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.FullName() == "(*sync.Pool).Put"
}

// constant reports whether e is a constant expression, which the compiler
// stores once in read-only data.
func constant(pass *analysis.Pass, e ast.Expr) bool {
	return pass.TypesInfo.Types[e].Value != nil
}

// indirect reports whether storing a value of type typ in an interface copies
// it into memory. The type parameter test comes first, because go/types panics
// on the size of a layout a type argument decides.
func indirect(typ types.Type, sizes types.Sizes) bool {
	if mentions(typ) {
		return false
	}
	switch u := typ.Underlying().(type) {
	case *types.Interface:
		// An interface reaches the pool as two words, whatever it holds.
		return false
	case *types.Basic:
		// An untyped nil has no type to size, and Put returns before storing
		// one.
		if u.Kind() == types.UntypedNil {
			return false
		}
	}
	// A value that occupies no bytes is stored without an allocation.
	return sizes.Sizeof(typ) != 0 && !pointerWord(typ, sizes)
}

// mentions reports whether the representation of typ depends on a type
// argument. A pointer, a slice, a map, a chan, a func, and an interface look
// the same whatever their type arguments are, so only an array or a struct can
// hide one.
func mentions(typ types.Type) bool {
	typ = types.Unalias(typ)
	if _, ok := typ.(*types.TypeParam); ok {
		return true
	}
	switch typ := typ.Underlying().(type) {
	case *types.Array:
		return mentions(typ.Elem())
	case *types.Struct:
		for field := range typ.Fields() {
			if mentions(field.Type()) {
				return true
			}
		}
	}
	return false
}

// pointerWord reports whether typ, which occupies at least one byte, is one
// word that a collector reads as a pointer.
func pointerWord(typ types.Type, sizes types.Sizes) bool {
	switch typ := typ.Underlying().(type) {
	case *types.Pointer, *types.Chan, *types.Map, *types.Signature:
		return true
	case *types.Basic:
		return typ.Kind() == types.UnsafePointer
	case *types.Struct:
		last := typ.Field(typ.NumFields() - 1).Type()
		return sizes.Sizeof(typ) == sizes.Sizeof(last) &&
			pointerWord(last, sizes)
	case *types.Array:
		return typ.Len() == 1 && pointerWord(typ.Elem(), sizes)
	}
	return false
}
