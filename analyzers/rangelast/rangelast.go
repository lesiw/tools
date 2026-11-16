// Package rangelast provides an analysis.Analyzer that reports a range loop
// that keeps only the last element of a slice or array.
//
// The specification's "For statements with range clause" section
// (https://go.dev/ref/spec#For_range) says that a range over an array or slice
// produces the elements in increasing index order, from 0 to len(x)-1. A loop
// whose body only assigns the range value to a variable sets that variable to
// the last element, and x[len(x)-1] reads the same element in one expression.
//
// The loop is reported when its body is one assignment of the range value, the
// range clause declares that value with :=, and the range expression is an
// array or a slice. A pointer to an array counts as an array, and a type
// parameter counts when every type in its constraint is one of those. A
// string is left alone because a range yields runes where an index yields
// bytes. A map has no index order, a channel has no index, and a loop that
// uses its key is not reported.
//
// The target is a variable or a field path whose bases are struct values. An
// element, a dereference, and a field reached through a pointer can write a
// location the range reads later, so they are left alone. A target that refers
// to the range value is left alone too. A blank target is left alone.
//
// The fix replaces the loop with target = x[len(x)-1]. It is withheld unless x
// is known to be non-empty: an array of positive length, a slice literal with
// an element, and a type parameter whose every type is non-empty qualify. A
// known-empty value is left alone: an array of length zero, a pointer to one,
// a slice literal with no elements, or a type parameter whose every type is
// empty. A slice whose length the rule does not prove is reported without a
// fix, since an empty range leaves target as it was and an out-of-range index
// panics (https://go.dev/ref/spec#Index_expressions). The fix is withheld when
// evaluating the range expression twice can give a different value, since the
// loop evaluates it once and the replacement evaluates it twice. It is
// withheld when a comment sits inside the loop, since the replacement would
// remove the comment, when the replacement would span lines, and when len
// refers to a declaration instead of the builtin.
//
// The rule is original to this suite. The README catalogs every check,
// exemption, and fix.
package rangelast

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "rangelast",
	Doc:  "reports a range loop that keeps only the last element",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if loop, ok := n.(*ast.RangeStmt); ok {
				check(pass, file, loop)
			}
			return true
		})
	}
	return nil, nil
}

// check reports loop when its body keeps only the range's last element.
func check(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt) {
	lhs, ok := destination(pass, loop)
	if !ok {
		return
	}
	expr := ast.Unparen(loop.X)
	nonEmpty, empty, ok := container(pass, expr)
	if !ok || empty {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:            loop.Pos(),
		End:            loop.End(),
		Message:        "loop only keeps the last element",
		SuggestedFixes: fix(pass, file, loop, lhs, expr, nonEmpty),
	})
}

// destination returns the variable that loop assigns the range value to, when
// loop has the reported form.
func destination(pass *analysis.Pass, loop *ast.RangeStmt) (ast.Expr, bool) {
	value, assign, ok := single(loop)
	if !ok {
		return nil, false
	}
	rhs, ok := ast.Unparen(assign.Rhs[0]).(*ast.Ident)
	if !ok || pass.TypesInfo.ObjectOf(rhs) != pass.TypesInfo.ObjectOf(value) {
		return nil, false
	}
	lhs := ast.Unparen(assign.Lhs[0])
	obj, ok := variable(pass, lhs)
	if !ok || obj == pass.TypesInfo.ObjectOf(value) {
		return nil, false
	}
	if astexpr.Mentions(pass, lhs, value) {
		return nil, false
	}
	return lhs, true
}

// variable returns the variable that e refers to, for an identifier, a
// package-qualified name, or a field path whose bases are struct values.
func variable(pass *analysis.Pass, e ast.Expr) (*types.Var, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if e.Name == "_" {
			return nil, false
		}
		v, ok := pass.TypesInfo.ObjectOf(e).(*types.Var)
		return v, ok
	case *ast.SelectorExpr:
		if qual, ok := ast.Unparen(e.X).(*ast.Ident); ok {
			if _, ok := pass.TypesInfo.Uses[qual].(*types.PkgName); ok {
				v, ok := pass.TypesInfo.Uses[e.Sel].(*types.Var)
				return v, ok
			}
		}
		if sel := pass.TypesInfo.Selections[e]; sel != nil && sel.Indirect() {
			return nil, false
		}
		base := ast.Unparen(e.X)
		if !structValue(pass.TypesInfo.TypeOf(base)) {
			return nil, false
		}
		if _, ok := variable(pass, base); !ok {
			return nil, false
		}
		v, ok := pass.TypesInfo.Uses[e.Sel].(*types.Var)
		return v, ok
	}
	return nil, false
}

// single returns the assignment in loop's body and the range value, when the
// clause declares the value and leaves the key blank.
func single(loop *ast.RangeStmt) (*ast.Ident, *ast.AssignStmt, bool) {
	if loop.Tok != token.DEFINE || loop.Value == nil {
		return nil, nil, false
	}
	value, ok := loop.Value.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	if loop.Key != nil {
		key, ok := loop.Key.(*ast.Ident)
		if !ok || key.Name != "_" {
			return nil, nil, false
		}
	}
	if len(loop.Body.List) != 1 {
		return nil, nil, false
	}
	assign, ok := loop.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	return value, assign, true
}

// container reports whether expr is a slice or array, whether every value it
// can hold has an element, and whether it is known to be empty.
func container(pass *analysis.Pass, expr ast.Expr) (nonEmpty, empty, ok bool) {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false, false, false
	}
	ok, terms := containers(t)
	if !ok {
		return false, false, false
	}
	nonEmpty, empty = classify(terms)
	if lit, ok := ast.Unparen(expr).(*ast.CompositeLit); ok {
		if _, ok := t.Underlying().(*types.Slice); ok {
			nonEmpty = len(lit.Elts) > 0
			empty = len(lit.Elts) == 0
		}
	}
	return nonEmpty, empty, true
}

// classify reports whether every type in terms is non-empty, and whether every
// one is empty. A slice has no length the rule can prove.
func classify(terms []types.Type) (nonEmpty, empty bool) {
	if len(terms) == 0 {
		return false, true
	}
	nonEmpty, empty = true, true
	for _, t := range terms {
		n, ok := arrayLength(t)
		if !ok {
			return false, false
		}
		if n == 0 {
			nonEmpty = false
		} else {
			empty = false
		}
	}
	return nonEmpty, empty
}

// arrayLength returns the length of an array or pointer to an array.
func arrayLength(t types.Type) (int64, bool) {
	switch t := types.Unalias(t).Underlying().(type) {
	case *types.Array:
		return t.Len(), true
	case *types.Pointer:
		array, ok := types.Unalias(t.Elem()).Underlying().(*types.Array)
		if !ok {
			return 0, false
		}
		return array.Len(), true
	}
	return 0, false
}

// containers reports whether every type in t's type set is a slice or array,
// and returns the underlying types that survive.
func containers(t types.Type) (ok bool, terms []types.Type) {
	switch t := t.(type) {
	case *types.TypeParam:
		return containers(t.Constraint())
	case *types.Union:
		if t.Len() == 0 {
			return false, nil
		}
		for term := range t.Terms() {
			ok, sub := containers(term.Type())
			if !ok {
				return false, nil
			}
			terms = unionTerms(terms, sub)
		}
		return true, terms
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		var found, filter bool
		for embedded := range iface.EmbeddedTypes() {
			if comparableOnly(embedded) {
				filter = true
				continue
			}
			if unrestricted(embedded) {
				continue
			}
			ok, sub := containers(embedded)
			if !ok {
				return false, nil
			}
			if found {
				terms = intersectTerms(terms, sub)
			} else {
				terms, found = sub, true
			}
		}
		if filter {
			terms = onlyComparable(terms)
		}
		return found, terms
	}
	return concrete(t)
}

// unionTerms returns the types that appear in either set.
func unionTerms(a, b []types.Type) []types.Type {
	out := append([]types.Type(nil), a...)
	for _, t := range b {
		out = addTerm(out, t)
	}
	return out
}

// intersectTerms returns the types that appear in both sets.
func intersectTerms(a, b []types.Type) (out []types.Type) {
	for _, x := range a {
		for _, y := range b {
			if sameType(x, y) {
				out = addTerm(out, x)
			}
		}
	}
	return out
}

// addTerm appends t unless an identical type is already present.
func addTerm(terms []types.Type, t types.Type) []types.Type {
	for _, have := range terms {
		if sameType(have, t) {
			return terms
		}
	}
	return append(terms, t)
}

// sameType reports whether a and b have the same underlying type.
func sameType(a, b types.Type) bool {
	return types.Identical(
		types.Unalias(a).Underlying(),
		types.Unalias(b).Underlying(),
	)
}

// structValue reports whether t's underlying type is a struct.
func structValue(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

// concrete reports whether t is a slice or array, and returns its type.
func concrete(t types.Type) (ok bool, terms []types.Type) {
	switch t := types.Unalias(t).Underlying().(type) {
	case *types.Slice, *types.Array:
		return true, []types.Type{t}
	case *types.Pointer:
		if _, ok := types.Unalias(t.Elem()).Underlying().(*types.Array); ok {
			return true, []types.Type{t}
		}
	}
	return false, nil
}

// comparableOnly reports whether t restricts a type set to comparable types
// and nothing else, as comparable does, including through a named interface.
func comparableOnly(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok || !iface.IsComparable() || iface.NumMethods() != 0 {
		return false
	}
	for embedded := range iface.EmbeddedTypes() {
		if !comparableOnly(embedded) {
			return false
		}
	}
	return true
}

// onlyComparable returns the terms that are comparable.
func onlyComparable(terms []types.Type) (out []types.Type) {
	for _, t := range terms {
		if types.Comparable(t) {
			out = addTerm(out, t)
		}
	}
	return out
}

// unrestricted reports whether an embedded type leaves the underlying kind and
// length unconstrained, as comparable or any does, including through a named
// interface.
func unrestricted(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for embedded := range iface.EmbeddedTypes() {
		if !unrestricted(embedded) {
			return false
		}
	}
	return true
}

// fix returns the edit that replaces loop with one assignment of the last
// element, when the value is known not to be empty and the edit loses nothing.
func fix(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt, lhs, expr ast.Expr, nonEmpty bool) []analysis.SuggestedFix {
	if !nonEmpty || astexpr.Effectful(pass, expr) {
		return nil
	}
	if commented(file, loop) || !lenBuiltin(pass, file, loop.Pos()) {
		return nil
	}
	text := astexpr.Text(lhs) + " = " + index(expr)
	if strings.ContainsRune(text, '\n') {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message: "Replace the loop with " + text,
		TextEdits: []analysis.TextEdit{{
			Pos:     loop.Pos(),
			End:     loop.End(),
			NewText: []byte(text),
		}},
	}}
}

// index returns x[len(x)-1] for x.
func index(x ast.Expr) string {
	length := &ast.CallExpr{
		Fun:  ast.NewIdent("len"),
		Args: []ast.Expr{x},
	}
	return astexpr.Text(&ast.IndexExpr{
		X: x,
		Index: &ast.BinaryExpr{
			X:  length,
			Op: token.SUB,
			Y:  &ast.BasicLit{Kind: token.INT, Value: "1"},
		},
	})
}

// lenBuiltin reports whether len refers to the predeclared function at pos. A
// shadowed len would resolve the call to itself.
func lenBuiltin(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	root := pass.TypesInfo.Scopes[file]
	if root == nil {
		return false
	}
	_, obj := root.Innermost(pos).LookupParent("len", pos)
	return obj == types.Universe.Lookup("len")
}

// commented reports whether a comment lies inside loop.
func commented(file *ast.File, loop *ast.RangeStmt) bool {
	for _, group := range file.Comments {
		if group.Pos() > loop.Pos() && group.End() < loop.End() {
			return true
		}
	}
	return false
}
