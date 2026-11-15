package evalorder

import (
	"go/ast"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

type kind int

const (
	field kind = iota
	element
	reference
)

// step is one move into the storage a place names: a field, an element, or the
// data a pointer or slice refers to.
type step struct {
	kind kind

	// key is the index of a field, which is how two spellings of one field
	// meet, or the constant index of an element. An element whose key is
	// empty is an index that is not a constant.
	key string
}

// through is the step that crosses from a value to the data it names.
var through = step{kind: reference}

// same reports whether two steps name the same move. An element whose index
// is not a constant matches every element, since two variables can hold the
// same index.
func (s step) same(other step) bool {
	if s.kind != other.kind {
		return false
	}
	if s.kind != element {
		return s.key == other.key
	}
	return s.key == other.key || s.key == "" || other.key == ""
}

// place names a piece of storage: the expression that names it, the variable
// it starts at, and the fields, elements and references that lead to it from
// there.
type place struct {
	expr ast.Expr
	v    *types.Var
	path []step
}

// placeOf returns the storage expr names.
func placeOf(pass *analysis.Pass, expr ast.Expr) (place, bool) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		v, ok := pass.TypesInfo.Uses[expr].(*types.Var)
		return place{expr: expr, v: v}, ok
	case *ast.SelectorExpr:
		base, ok := placeOf(pass, expr.X)
		if !ok {
			return qualified(pass, expr)
		}
		selection, ok := pass.TypesInfo.Selections[expr]
		if !ok || selection.Kind() != types.FieldVal {
			// A method value names no field; it holds its receiver, which the
			// field steps of the selection lead to.
			var (
				receiver = ast.Unparen(expr.X)
				indexes  = selection.Index()
			)
			if len(indexes) > 0 {
				indexes = indexes[:len(indexes)-1] // the method is last
			}
			steps := fieldSteps(pass.TypesInfo.TypeOf(expr.X), indexes)
			path := with(referring(pass, base, expr.X), steps...)
			return place{expr: receiver, v: base.v, path: path}, true
		}
		path := referring(pass, base, expr.X)
		steps := fieldSteps(pass.TypesInfo.TypeOf(expr.X), selection.Index())
		return place{expr: expr, v: base.v, path: with(path, steps...)}, true
	case *ast.IndexExpr:
		base, ok := placeOf(pass, expr.X)
		if !ok {
			return place{}, false
		}
		next := step{kind: element, key: indexName(pass, expr.Index)}
		path := with(referring(pass, base, expr.X), next)
		return place{expr: expr, v: base.v, path: path}, true
	case *ast.StarExpr:
		base, ok := placeOf(pass, expr.X)
		if !ok {
			return place{}, false
		}
		return place{
			expr: expr,
			v:    base.v,
			path: with(base.path, through),
		}, true
	}
	return place{}, false
}

// qualified returns the storage a selector names when its base is a package:
// a package-qualified variable is a place of its own.
func qualified(pass *analysis.Pass, sel *ast.SelectorExpr) (place, bool) {
	pkg, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return place{}, false
	}
	if _, ok := pass.TypesInfo.Uses[pkg].(*types.PkgName); !ok {
		return place{}, false
	}
	v, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	return place{expr: sel, v: v}, ok
}

// fieldSteps returns the field steps of an index chain, crossing a reference
// for each pointer the fields are reached through.
func fieldSteps(recv types.Type, indexes []int) (steps []step) {
	t := recv
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem() // referring has crossed the receiver already
	}
	for _, index := range indexes {
		for {
			ptr, ok := t.Underlying().(*types.Pointer)
			if !ok {
				break
			}
			steps = append(steps, through)
			t = ptr.Elem()
		}
		strct, ok := t.Underlying().(*types.Struct)
		if !ok {
			return steps
		}
		t = strct.Field(index).Type()
		steps = append(steps, step{kind: field, key: strconv.Itoa(index)})
	}
	return steps
}

// referring returns base's path, with the step that crosses to the data src
// names when src refers to data that lives elsewhere.
func referring(pass *analysis.Pass, base place, src ast.Expr) []step {
	if refers(pass.TypesInfo.TypeOf(src)) {
		return with(base.path, through)
	}
	return base.path
}

// with returns a copy of path with steps appended, so that places never share
// a path's array.
func with(path []step, steps ...step) []step {
	out := make([]step, 0, len(path)+len(steps))
	out = append(out, path...)
	return append(out, steps...)
}

// indices returns the index expressions in the chain expr names.
func indices(expr ast.Expr) []ast.Expr {
	switch e := ast.Unparen(expr).(type) {
	case *ast.IndexExpr:
		return append(indices(e.X), e.Index)
	case *ast.SelectorExpr:
		return indices(e.X)
	case *ast.StarExpr:
		return indices(e.X)
	}
	return nil
}

// indexName returns the constant value of an index expression, and the empty
// name when the index is not a constant.
func indexName(pass *analysis.Pass, index ast.Expr) string {
	if tv, ok := pass.TypesInfo.Types[index]; ok && tv.Value != nil {
		return tv.Value.ExactString()
	}
	return ""
}

// mayWrite reports whether a callee given the address at w can change what a
// read at r sees. A write reaches a read that is inside it. A read reaches a
// write that is inside the read's own storage, and not a write that follows a
// reference out of it: reading a pointer does not read the data it names.
func mayWrite(w, r place) bool {
	if w.v != r.v {
		return false
	}
	if prefix(w.path, r.path) {
		return true
	}
	if !prefix(r.path, w.path) {
		return false
	}
	return !slices.Contains(w.path[len(r.path):], through)
}

// prefix reports whether a is a prefix of b.
func prefix(a, b []step) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if !a[i].same(b[i]) {
			return false
		}
	}
	return true
}

// span is a range of source.

// isPointer reports whether t is a pointer type.
func isPointer(t types.Type) bool {
	_, ok := t.Underlying().(*types.Pointer)
	return ok
}

// reached returns the places that naming an address reads: a pointer the
// address is taken through, and a slice or map whose element it is.
func reached(pass *analysis.Pass, expr ast.Expr) (out []place) {
	for {
		var base ast.Expr
		switch e := ast.Unparen(expr).(type) {
		case *ast.SelectorExpr:
			base = e.X
		case *ast.IndexExpr:
			base = e.X
		case *ast.StarExpr:
			base = e.X
		default:
			return out
		}
		if refers(pass.TypesInfo.TypeOf(base)) {
			if p, ok := placeOf(pass, base); ok {
				out = append(out, p)
			}
		}
		expr = base
	}
}

// isArray reports whether a value of type t is an array or a pointer to one.
func isArray(t types.Type) bool {
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}
	_, ok := t.Underlying().(*types.Array)
	return ok
}

// refers reports whether a value of type t refers to data that lives
// elsewhere. A write below a pointer or a slice is not a write to the variable
// that holds it.
func refers(t types.Type) bool {
	if param, ok := t.(*types.TypeParam); ok {
		return constraintRefers(param.Constraint())
	}
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Slice:
		return true
	}
	return false
}

// constraintRefers reports whether every type the constraint c admits refers
// to data that lives elsewhere.
func constraintRefers(c types.Type) bool {
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if !refers(term.Type()) {
				return false
			}
		}
		return true
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return refers(c) // a bare type term is embedded as the type itself.
	}
	var admitted int
	for e := range iface.EmbeddedTypes() {
		if neutral(e) {
			continue // it admits every type, so it takes nothing away
		}
		if !constraintRefers(e) {
			return false
		}
		admitted++
	}
	return admitted > 0
}

// neutral reports whether c says nothing about the storage of a type it
// admits. The empty interface and comparable both admit types that do not
// refer, and neither narrows a type set that another element of an
// intersection already names.
func neutral(c types.Type) bool {
	iface, ok := c.Underlying().(*types.Interface)
	if !ok || iface.NumMethods() > 0 {
		return false
	}
	return iface.IsMethodSet() || iface.IsComparable()
}
