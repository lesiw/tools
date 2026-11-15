// Package astexpr holds the facts about an expression that the analyzers
// share: how it prints, whether two of them read the same value, whether
// reading one twice gives one value, and whether its type can hold a NaN.
// Package astexpr holds the facts about the syntax tree that the analyzers
// share: an expression's printed form, whether reading it twice gives one
// value, whether its type can hold a NaN, whether its type set holds only
// arrays, and whether a statement reads the value it writes.
package astexpr

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Text returns e as source text, with the line breaks the printer chooses. A
// fresh FileSet drops the line breaks of the source, so a wrapped expression
// matches an unwrapped one, and go/printer keeps the elements of a composite
// literal, which types.ExprString would elide.
func Text(e ast.Expr) string {
	var buf bytes.Buffer
	// A bytes.Buffer cannot fail.
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
}

// Pure reports whether evaluating call twice gives the same value: a
// conversion does, and so does a builtin that only reads its arguments.
func Pure(pass *analysis.Pass, call *ast.CallExpr) bool {
	return pureCall(pass, call)
}

// OneLine returns e as source text on one line, and reports false when it has
// none, as a raw string holding a newline has none.
func OneLine(e ast.Expr) (string, bool) {
	text := Text(e)
	if strings.ContainsRune(text, '\n') {
		return "", false
	}
	return text, true
}

// Same reports whether a and b read the same value: the same identifiers and
// fields by object, and the same indexes, literals, and dereferences. A name
// is compared by the object it names, so a shadowed one is a different value.
// A call is left out, since two of them may differ.
func Same(pass *analysis.Pass, a, b ast.Expr) bool {
	a, b = ast.Unparen(a), ast.Unparen(b)
	switch a := a.(type) {
	case *ast.Ident:
		ident, ok := b.(*ast.Ident)
		if !ok {
			return false
		}
		obj := pass.TypesInfo.ObjectOf(a)
		return obj != nil && obj == pass.TypesInfo.ObjectOf(ident)
	case *ast.SelectorExpr:
		sel, ok := b.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		obj := pass.TypesInfo.ObjectOf(a.Sel)
		return obj != nil && obj == pass.TypesInfo.ObjectOf(sel.Sel) &&
			Same(pass, a.X, sel.X)
	case *ast.IndexExpr:
		index, ok := b.(*ast.IndexExpr)
		return ok && Same(pass, a.X, index.X) &&
			Same(pass, a.Index, index.Index)
	case *ast.StarExpr:
		star, ok := b.(*ast.StarExpr)
		return ok && Same(pass, a.X, star.X)
	case *ast.BasicLit:
		lit, ok := b.(*ast.BasicLit)
		return ok && a.Kind == lit.Kind && a.Value == lit.Value
	}
	return false
}

// Mentions reports whether e mentions s anywhere, in any form.
func Mentions(pass *analysis.Pass, e ast.Node, s ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if expr, ok := n.(ast.Expr); ok && Same(pass, expr, s) {
			found = true
		}
		return !found
	})
	return found
}

// Effectful reports whether reading e can give a different value the second
// time.
func Effectful(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if !pureCall(pass, n) {
				found = true
			}
		case *ast.UnaryExpr:
			switch n.Op {
			case token.ARROW:
				found = true // a receive takes an element
			case token.AND:
				// An address of a literal is a new value each time.
				found = found || hasLiteral(n.X)
			}
		}
		return !found
	})
	return found
}

// hasLiteral reports whether e contains a composite literal.
func hasLiteral(e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.CompositeLit); ok {
			found = true
		}
		return !found
	})
	return found
}

// pureCall reports whether evaluating call twice gives the same value. A
// conversion does, and so does a builtin that only reads its arguments.
func pureCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if tv, ok := pass.TypesInfo.Types[call.Fun]; ok && tv.IsType() {
		return true
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	// The allowance is closed: a builtin added to the language counts as
	// effectful until it is listed here.
	switch pass.TypesInfo.Uses[ident] {
	case types.Universe.Lookup("cap"), types.Universe.Lookup("complex"),
		types.Universe.Lookup("imag"), types.Universe.Lookup("len"),
		types.Universe.Lookup("max"), types.Universe.Lookup("min"),
		types.Universe.Lookup("real"):
		return true
	}
	return false
}

// MayBeNaN reports whether a value of type t can be a NaN. An interface can
// hold one, and a type parameter is left to its constraint.
func MayBeNaN(t types.Type) bool {
	if param, ok := t.(*types.TypeParam); ok {
		return constraintNaN(param.Constraint())
	}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		return t.Info()&(types.IsFloat|types.IsComplex) != 0
	case *types.Interface:
		return true
	case *types.Struct:
		for field := range t.Fields() {
			if MayBeNaN(field.Type()) {
				return true
			}
		}
	case *types.Array:
		return MayBeNaN(t.Elem())
	}
	return false
}

// constraintNaN reports whether the type set of c holds a type that can be a
// NaN. The elements of an interface are intersected, so one element that rules
// a NaN out settles the constraint.
func constraintNaN(c types.Type) bool {
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if MayBeNaN(term.Type()) {
				return true
			}
		}
		return false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return MayBeNaN(c)
	}
	for embedded := range iface.EmbeddedTypes() {
		if !constraintNaN(embedded) {
			return false
		}
	}
	// No element rules a NaN out, so any type with the methods fits.
	return true
}

// IsArray reports whether every type in t's type set is an array. A lone array
// qualifies, and so does a type parameter whose constraint holds only arrays.
// A constraint that names no type of its own, as comparable does, leaves the
// set as it was, and a set that names none is no array.
func IsArray(t types.Type) (found bool) {
	switch t := t.(type) {
	case *types.Union:
		if t.Len() == 0 {
			return false
		}
		for term := range t.Terms() {
			if !IsArray(term.Type()) {
				return false
			}
		}
		return true
	case *types.TypeParam:
		return IsArray(t.Constraint())
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		_, array := t.Underlying().(*types.Array)
		return array
	}
	for embedded := range iface.EmbeddedTypes() {
		if unrestricted(embedded) {
			continue
		}
		if !IsArray(embedded) {
			return false
		}
		found = true
	}
	return found
}

// unrestricted reports whether an element of a type set names no type of its
// own, as an embedding of comparable or any does.
func unrestricted(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.NumEmbeddeds() == 0
}

// SelfReads reports whether a statement reads the value it writes, as an
// increment or a compound assignment does.
func SelfReads(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.IncDecStmt:
		return true
	case *ast.AssignStmt:
		// A short declaration writes a new variable, so there is no value to
		// read.
		return stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE
	}
	return false
}
