// Package nilsliceappend provides an analysis.Analyzer that reports an append
// to a nil slice where a slice literal gives the result.
//
// The section of the spec on appending to and copying slices says append
// "appends zero or more values x to a slice s of type S and returns the
// resulting slice, also of type S"
// (https://go.dev/ref/spec#Appending_and_copying_slices). With a nil s the
// result holds only the appended values, which a slice literal gives directly.
//
// The first argument may be a nil slice of a named type, as in Bytes(nil), or
// a type parameter whose type set holds one underlying slice type. Only a
// direct conversion of nil is reported; a nil slice from a call, a slice
// expression, or a variable is left alone. A base written as make([]T, 0) or
// []T{} is not nil, so the call already starts from a fresh slice and is left
// alone. A spread append is left alone, since an empty source can make the
// result nil and no slice literal is nil. An append with no value after the
// nil slice is left alone for the same reason. A call with a comment inside it
// is reported without a fix, since the edit would delete the comment. A
// control clause gets parentheses around a literal of a named type, where its
// brace would read as the block. The literal's capacity is its length, where
// append may allocate more.
//
// The rule is adapted from staticcheck's S1019, which simplifies a make call
// by omitting redundant arguments.
package nilsliceappend

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilsliceappend",
	Doc:  "reports an append to a nil slice where a slice literal gives the result",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			call, ok := n.(*ast.CallExpr)
			if !ok || !appended(pass, call) || len(call.Args) < 2 {
				return true
			}
			if call.Ellipsis.IsValid() {
				return true
			}
			typ, ok := nilSlice(pass, call.Args[0])
			if !ok {
				return true
			}
			report(pass, file, call, typ, header(stack))
			return true
		})
	}
	return nil, nil
}

// appended reports whether call is the builtin append.
func appended(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("append")
}

// nilSlice returns the type expression of a conversion to a nil slice.
func nilSlice(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	if tv, ok := pass.TypesInfo.Types[call.Fun]; !ok || !tv.IsType() {
		return nil, false
	}
	id, ok := ast.Unparen(call.Args[0]).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("nil") {
		return nil, false
	}
	typ := pass.TypesInfo.TypeOf(call)
	if typ == nil || !sliceType(typ) {
		return nil, false
	}
	return ast.Unparen(call.Fun), true
}

// sliceType reports whether t is a slice or a type parameter whose type set
// holds one underlying slice type.
func sliceType(t types.Type) bool {
	t = types.Unalias(t)
	if param, ok := t.(*types.TypeParam); ok {
		return constraintSlice(param.Constraint())
	}
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// constraintSlice reports whether the type set of c holds one underlying
// slice type.
func constraintSlice(c types.Type) bool {
	terms, universal := constraintTerms(c)
	if universal || len(terms) == 0 {
		return false
	}
	var underlying types.Type
	for _, term := range terms {
		typ := term.typ
		if term.exact {
			typ = typ.Underlying()
		}
		slice, ok := typ.(*types.Slice)
		if !ok {
			return false
		}
		if underlying == nil {
			underlying = slice
		} else if !types.Identical(underlying, slice) {
			return false
		}
	}
	return true
}

// term is one element of a type set. An exact term names one type, and the
// type of an approximate term is its underlying type.
type term struct {
	typ   types.Type
	exact bool
}

// constraintTerms returns the type terms of c, and whether c supplies no type
// terms at all. Embedded elements are intersected.
func constraintTerms(c types.Type) ([]term, bool) {
	c = types.Unalias(c)
	if union, ok := c.(*types.Union); ok {
		var out []term
		for t := range union.Terms() {
			if t.Tilde() {
				out = append(out, term{typ: t.Type().Underlying()})
				continue
			}
			terms, universal := constraintTerms(t.Type())
			if universal {
				return nil, true
			}
			out = append(out, terms...)
		}
		return unique(out), false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return []term{{typ: c, exact: true}}, false
	}
	var terms []term
	universal := true
	for embedded := range iface.EmbeddedTypes() {
		narrowed, none := constraintTerms(embedded)
		if none {
			continue
		}
		if universal {
			terms, universal = narrowed, false
		} else {
			terms = intersect(terms, narrowed)
		}
	}
	return terms, universal
}

// intersect returns the terms shared by a and b.
func intersect(a, b []term) []term {
	var out []term
	for _, x := range a {
		for _, y := range b {
			if t, ok := meet(x, y); ok {
				out = append(out, t)
			}
		}
	}
	return unique(out)
}

// meet returns the term common to a and b.
func meet(a, b term) (term, bool) {
	switch {
	case a.exact && b.exact:
		return a, types.Identical(a.typ, b.typ)
	case a.exact && !b.exact:
		return a, types.Identical(a.typ.Underlying(), b.typ)
	case !a.exact && b.exact:
		return b, types.Identical(a.typ, b.typ.Underlying())
	}
	return a, types.Identical(a.typ, b.typ)
}

// unique drops duplicate terms.
func unique(terms []term) (out []term) {
	for _, t := range terms {
		var dup bool
		for _, u := range out {
			if t.exact == u.exact && types.Identical(t.typ, u.typ) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, t)
		}
	}
	return
}

// report suggests the slice literal that holds the appended values.
func report(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, typ ast.Expr, header bool) {
	diag := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "append to a nil slice; write the slice literal",
	}
	if !commented(file, call) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace the call with the slice literal",
			TextEdits: []analysis.TextEdit{{
				Pos:     call.Pos(),
				End:     call.End(),
				NewText: []byte(literal(call.Args[1:], typ, header)),
			}},
		}}
	}
	pass.Report(diag)
}

// literal returns the slice literal holding args. A named type in a control
// clause takes parentheses, where its brace would read as the block.
func literal(args []ast.Expr, typ ast.Expr, header bool) string {
	values := make([]string, len(args))
	for i, arg := range args {
		values[i] = astexpr.Text(arg)
	}
	lit := astexpr.Text(typ) + "{" + strings.Join(values, ", ") + "}"
	if header && typeName(typ) {
		return "(" + lit + ")"
	}
	return lit
}

// typeName reports whether t is a type name, with type arguments if any.
func typeName(t ast.Expr) bool {
	switch ast.Unparen(t).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return true
	}
	return false
}

// header reports whether stack holds call in the header of an if, for, range,
// or switch statement, outside any call argument, parenthesized expression, or
// other delimited subexpression.
func header(stack []ast.Node) bool {
	for i := len(stack) - 1; i > 0; i-- {
		parent, child := stack[i-1], stack[i]
		switch n := parent.(type) {
		case *ast.IfStmt:
			return before(child, n.Body.Lbrace)
		case *ast.ForStmt:
			return before(child, n.Body.Lbrace)
		case *ast.RangeStmt:
			return before(child, n.Body.Lbrace)
		case *ast.SwitchStmt:
			return before(child, n.Body.Lbrace)
		case *ast.TypeSwitchStmt:
			return before(child, n.Body.Lbrace)
		case *ast.BlockStmt, *ast.ParenExpr, *ast.CompositeLit:
			return false
		case *ast.CallExpr:
			if child != n.Fun {
				return false
			}
		case *ast.IndexExpr:
			if child != n.X {
				return false
			}
		case *ast.SliceExpr:
			if child != n.X {
				return false
			}
		case *ast.TypeAssertExpr:
			if child != n.X {
				return false
			}
		case *ast.FuncLit:
			if child == n.Body {
				return false
			}
		}
	}
	return false
}

// before reports whether child ends before brace.
func before(child ast.Node, brace token.Pos) bool {
	return child.End() <= brace
}

// commented reports whether a comment lies inside call, where the fix would
// delete it.
func commented(file *ast.File, call *ast.CallExpr) bool {
	for _, group := range file.Comments {
		if group.Pos() > call.Pos() && group.End() < call.End() {
			return true
		}
	}
	return false
}
