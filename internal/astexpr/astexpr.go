// Package astexpr holds the facts about the syntax tree that the analyzers
// share: an expression's printed form, whether two expressions read the same
// value or can be evaluated twice, whether evaluating one can run code or
// panic, whether a type has no interface in it, whether a selection goes
// through a pointer, whether a type can hold a NaN, whether its type set holds
// only arrays, and whether a statement reads the value it writes.
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

// NoInterface reports whether t has no interface in it, which could contain a
// value that cannot be hashed.
func NoInterface(t types.Type) bool {
	switch t := types.Unalias(t).Underlying().(type) {
	case *types.Interface:
		return false
	case *types.Struct:
		for field := range t.Fields() {
			if !NoInterface(field.Type()) {
				return false
			}
		}
	case *types.Array:
		return NoInterface(t.Elem())
	}
	return true
}

// Safe reports whether e can be evaluated without running code or panicking.
func Safe(pass *analysis.Pass, e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return Safe(pass, e.X)
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			if !Safe(pass, elt) {
				return false
			}
		}
		return true
	case *ast.KeyValueExpr:
		return Safe(pass, e.Key) && Safe(pass, e.Value)
	case *ast.SelectorExpr:
		if _, ok := pass.TypesInfo.Selections[e]; !ok {
			return Safe(pass, e.X)
		}
		return Follows(pass, e) && Safe(pass, e.X)
	case *ast.IndexExpr:
		// An index into a map cannot panic; one into a slice or a
		// string checks its bounds.
		m, ok := mapType(pass, e.X)
		return ok && NoInterface(m.Key()) && Safe(pass, e.X) &&
			Safe(pass, e.Index)
	case *ast.UnaryExpr:
		return e.Op != token.ARROW && e.Op != token.MUL && Safe(pass, e.X)
	case *ast.CallExpr:
		return called(pass, e)
	case *ast.BinaryExpr:
		return computed(pass, e)
	}
	return false
}

// mapType returns the map type of e.
func mapType(pass *analysis.Pass, e ast.Expr) (*types.Map, bool) {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return nil, false
	}
	m, ok := t.Underlying().(*types.Map)
	return m, ok
}

// called reports whether a call cannot run user code or panic: a conversion
// judged on what it converts, or a builtin that only reads its arguments.
func called(pass *analysis.Pass, e *ast.CallExpr) bool {
	if pass.TypesInfo.Types[e.Fun].IsType() {
		return converted(pass, e)
	}
	id, ok := ast.Unparen(e.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	if !ok {
		return false
	}
	switch builtin.Name() {
	case "cap", "len", "max", "min":
	default:
		return false
	}
	for _, arg := range e.Args {
		if !Safe(pass, arg) {
			return false
		}
	}
	return true
}

// converted reports whether a conversion cannot panic: nothing it converts
// can, and it does not build an array.
func converted(pass *analysis.Pass, e *ast.CallExpr) bool {
	if t := pass.TypesInfo.TypeOf(e); t == nil || !arrayless(t) {
		return false
	}
	for _, arg := range e.Args {
		if !Safe(pass, arg) {
			return false
		}
	}
	return true
}

// arrayless reports whether converting a slice to t cannot panic: a conversion
// to an array, or to a pointer to one, panics when the slice is shorter.
func arrayless(t types.Type) bool {
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	u := t.Underlying()
	if p, ok := u.(*types.Pointer); ok {
		u = p.Elem().Underlying()
	}
	_, array := u.(*types.Array)
	return !array
}

// Follows reports whether reading e dereferences no pointer: neither the value
// it starts from, nor a pointer-typed field on the way to the one selected.
func Follows(pass *analysis.Pass, e *ast.SelectorExpr) bool {
	t := pass.TypesInfo.TypeOf(e.X)
	if t == nil {
		return false
	}
	if _, ok := t.Underlying().(*types.Pointer); ok {
		return false
	}
	sel, ok := pass.TypesInfo.Selections[e]
	if !ok {
		return true
	}
	t = sel.Recv()
	index := sel.Index()
	for _, i := range index[:len(index)-1] {
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		field := st.Field(i)
		if _, ok := field.Type().Underlying().(*types.Pointer); ok {
			return false
		}
		t = field.Type()
	}
	return true
}

// computed reports whether a binary expression cannot panic.
func computed(pass *analysis.Pass, e *ast.BinaryExpr) bool {
	switch e.Op {
	case token.QUO, token.REM, token.SHL, token.SHR:
		// A zero divisor and a negative shift count both panic.
		return false
	}
	if e.Op == token.EQL || e.Op == token.NEQ {
		// Two interface values panic on comparison when both hold the
		// same type and that type cannot be hashed.
		if t := pass.TypesInfo.TypeOf(e.X); t == nil || !NoInterface(t) {
			return false
		}
		if t := pass.TypesInfo.TypeOf(e.Y); t == nil || !NoInterface(t) {
			return false
		}
	}
	return Safe(pass, e.X) && Safe(pass, e.Y)
}

func SamePure(pass *analysis.Pass, a, b ast.Expr) bool {
	a, b = ast.Unparen(a), ast.Unparen(b)
	if Text(a) != Text(b) {
		return false
	}
	// Equal text is one expression, so one effect check answers for both.
	return !Effectful(pass, a)
}
