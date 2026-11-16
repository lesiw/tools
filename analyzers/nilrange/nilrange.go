// Package nilrange provides an analysis.Analyzer that reports a nil check
// wrapped around a range loop over a slice or a map.
//
// The section of the spec on for statements says that "if the map is nil, the
// number of iterations is 0" (https://go.dev/ref/spec#For_statements), and the
// section on length and capacity says the length of a nil slice is 0
// (https://go.dev/ref/spec#Length_and_capacity). A range over a nil slice or
// map therefore runs its body zero times, so the check is redundant.
//
// The check is an if with no init and no else, whose condition compares a
// slice or map with nil using !=, and whose body is one range loop over that
// value. The loop may be labeled. Either operand of the comparison may be nil,
// and parentheses are allowed. The value must be a name or a selector path; an
// index, a dereference, and a call are left alone. A type parameter qualifies
// when its core type is a slice or a map. A channel is left alone, since a
// range over a nil channel blocks forever. So is a pointer to an array, since
// a range over a nil one ranges over the array's length and may panic when it
// reads an element, and a function, since a range over a nil one panics as
// well.
//
// The fix replaces the if statement with the loop. The fix is withheld when a
// comment outside the loop would be removed.
//
// The rule is adapted from staticcheck's S1031.
package nilrange

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilrange",
	Doc:  "reports a nil check around a range loop over a slice or a map",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.IfStmt); ok {
				check(pass, file, stmt)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a nil check that wraps the only statement of an if body, a
// range loop over the checked value.
func check(pass *analysis.Pass, file *ast.File, stmt *ast.IfStmt) {
	if stmt.Init != nil || stmt.Else != nil {
		return
	}
	x, ok := nilOperand(pass, stmt.Cond)
	if !ok {
		return
	}
	loop, inner, ok := onlyRange(stmt.Body)
	if !ok || !variable(x) || !astexpr.Same(pass, loop.X, x) {
		return
	}
	t := pass.TypesInfo.TypeOf(x)
	if t == nil || !rangeable(t) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     stmt.Pos(),
		End:     stmt.End(),
		Message: "nil check around range is unnecessary",
	}
	if canFix(file, stmt, inner) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Drop the nil check",
			TextEdits: []analysis.TextEdit{{
				Pos: stmt.Pos(),
				End: inner.Pos(),
			}, {
				Pos: inner.End(),
				End: stmt.End(),
			}},
		}}
	}
	pass.Report(diag)
}

// canFix reports whether the edits that replace stmt with inner preserve every
// comment.
func canFix(file *ast.File, stmt *ast.IfStmt, inner ast.Stmt) bool {
	return !commented(file, stmt.Pos(), inner.Pos()) &&
		!commented(file, inner.End(), stmt.End())
}

// nilOperand returns the value cond compares with nil using !=.
func nilOperand(pass *analysis.Pass, cond ast.Expr) (ast.Expr, bool) {
	cmp, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok || cmp.Op != token.NEQ {
		return nil, false
	}
	if isNil(pass, cmp.Y) {
		return ast.Unparen(cmp.X), true
	}
	if isNil(pass, cmp.X) {
		return ast.Unparen(cmp.Y), true
	}
	return nil, false
}

// isNil reports whether e is the predeclared nil.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("nil")
}

// variable reports whether e is a name or a selector path, which is evaluated
// once for the check and once for the range without a call or an index.
func variable(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return variable(e.X)
	}
	return false
}

// onlyRange returns the range loop that is the only statement of body, and the
// statement that contains it.
func onlyRange(body *ast.BlockStmt) (*ast.RangeStmt, ast.Stmt, bool) {
	if body == nil || len(body.List) != 1 {
		return nil, nil, false
	}
	var (
		root  = body.List[0]
		inner = root
	)
	for {
		if loop, ok := inner.(*ast.RangeStmt); ok {
			return loop, root, true
		}
		labeled, ok := inner.(*ast.LabeledStmt)
		if !ok {
			return nil, nil, false
		}
		inner = labeled.Stmt
	}
}

// rangeable reports whether a value of type t may be ranged over when it is
// nil without doing anything. A slice and a map qualify, and so does a type
// parameter whose core type is one of those.
func rangeable(t types.Type) bool {
	t = types.Unalias(t)
	if param, ok := t.(*types.TypeParam); ok {
		return constraint(param.Constraint())
	}
	switch t.Underlying().(type) {
	case *types.Slice, *types.Map:
		return true
	}
	return false
}

// constraint reports whether t's core type is a slice or a map.
func constraint(t types.Type) bool {
	t = types.Unalias(t)
	if param, ok := t.(*types.TypeParam); ok {
		return constraint(param.Constraint())
	}
	terms, universal := constraintTerms(t)
	if universal || len(terms) == 0 {
		return false
	}
	var core types.Type
	for _, term := range terms {
		typ := term.typ
		if term.exact {
			typ = typ.Underlying()
		}
		switch typ.(type) {
		case *types.Slice, *types.Map:
		default:
			return false
		}
		if core == nil {
			core = typ
		} else if !types.Identical(core, typ) {
			return false
		}
	}
	return true
}

// term is one element of a type set. An exact term is a single type, and an
// approximate term is its underlying type.
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
		return uniqueTerms(out), false
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
			terms = intersectTerms(terms, narrowed)
		}
	}
	return terms, universal
}

// intersectTerms returns the terms shared by a and b.
func intersectTerms(a, b []term) []term {
	var out []term
	for _, x := range a {
		for _, y := range b {
			if t, ok := meetTerms(x, y); ok {
				out = append(out, t)
			}
		}
	}
	return uniqueTerms(out)
}

// meetTerms returns the term common to a and b.
func meetTerms(a, b term) (term, bool) {
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

// uniqueTerms drops duplicate terms.
func uniqueTerms(terms []term) (out []term) {
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

// commented reports whether a comment overlaps the text from pos to end.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && pos < group.End() {
			return true
		}
	}
	return false
}
