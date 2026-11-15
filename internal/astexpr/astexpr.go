// Package astexpr holds the facts about an expression that the analyzers
// share: its printed form, whether reading it twice gives one value, and
// whether its type can hold a NaN.
package astexpr

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Text returns e printed on one line. A fresh FileSet drops the source line
// breaks, so a wrapped expression matches an unwrapped one, and go/printer
// keeps the elements of a composite literal, which types.ExprString would
// elide.
func Text(e ast.Expr) string {
	var buf bytes.Buffer
	// A bytes.Buffer cannot fail.
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
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
