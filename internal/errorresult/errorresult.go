// Package errorresult holds the error-order check shared by errorlast and
// errnotlast.
package errorresult

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Check reports the first error result of fn when the final result does not
// implement error.
func Check(pass *analysis.Pass, fn *ast.FuncType) {
	if fn.Results == nil || len(fn.Results.List) == 0 {
		return
	}
	results := fn.Results.List
	if ImplementsError(pass.TypesInfo.TypeOf(results[len(results)-1].Type)) {
		return
	}
	for _, field := range results {
		if !IsError(pass.TypesInfo.TypeOf(field.Type)) {
			continue
		}
		pass.Reportf(
			field.Type.Pos(), "%s is not the last result",
			types.ExprString(field.Type),
		)
		return
	}
}

// ImplementsError reports whether t implements error, whatever its name.
func ImplementsError(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if tp, ok := t.(*types.TypeParam); ok {
		return typeParamIsError(tp)
	}
	return types.Implements(t, errorInterfaceType())
}

// typeParamIsError reports whether every type in tp's type set implements
// error.
func typeParamIsError(tp *types.TypeParam) bool {
	if types.Implements(tp, errorInterfaceType()) {
		return true
	}
	iface, ok := tp.Constraint().Underlying().(*types.Interface)
	if !ok {
		return false
	}
	if types.Implements(iface, errorInterfaceType()) {
		return true
	}
	for embedded := range iface.EmbeddedTypes() {
		if typeSetIsError(embedded) {
			return true
		}
	}
	return false
}

// typeSetIsError reports whether every type in t's type set implements error.
func typeSetIsError(t types.Type) bool {
	switch t := t.(type) {
	case *types.Union:
		var found bool
		for term := range t.Terms() {
			found = true
			if term.Tilde() || !typeSetIsError(term.Type()) {
				return false
			}
		}
		return found
	case *types.Interface:
		if types.Implements(t, errorInterfaceType()) {
			return true
		}
		for embedded := range t.EmbeddedTypes() {
			if typeSetIsError(embedded) {
				return true
			}
		}
		return false
	default:
		if types.Implements(t, errorInterfaceType()) {
			return true
		}
		iface, ok := t.Underlying().(*types.Interface)
		return ok && typeSetIsError(iface)
	}
}

// errorInterfaceType returns the predeclared error's underlying interface.
func errorInterfaceType() *types.Interface {
	return types.Universe.Lookup("error").
		Type().Underlying().(*types.Interface)
}

// IsError reports whether t is the type of an error result.
func IsError(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	errorType := types.Universe.Lookup("error").Type()
	// The predeclared error has a lowercase name, so the name test below
	// would miss it.
	if types.Identical(t, errorType) {
		return true
	}
	// A type parameter is left alone as an earlier result, whatever its
	// constraint. ImplementsError accepts a final one whose type set is
	// wholly error.
	if _, ok := t.(*types.TypeParam); ok {
		return false
	}
	err, ok := errorType.Underlying().(*types.Interface)
	if !ok || !types.Implements(t, err) {
		return false
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	n, ok := t.(*types.Named)
	return ok && strings.HasSuffix(n.Obj().Name(), "Error")
}
