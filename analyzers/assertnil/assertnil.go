// Package assertnil provides an analysis.Analyzer that reports a redundant nil
// check on the result of a type assertion.
//
// The Go FAQ says an interface value is nil only when its type and value are
// both unset. A successful assertion to an interface type yields the source's
// dynamic type, so the result is never nil.
//
// The rule is like staticcheck's S1020, Omit redundant nil check in type
// assertion: S1020 tests the value the assertion is applied to, and this tests
// the result.
//
// The fix drops the comparison. It is withheld where the edit would take a
// comment with it, or leave code the compiler rejects. The README catalogs
// every check, exemption, and fix.
package assertnil

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "assertnil",
	Doc:  "reports a redundant nil check on the result of a type assertion",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			check(pass, file, ifStmt)
			return true
		})
	}
	return nil, nil
}

// check reports a nil comparison in the condition of ifStmt that the
// assertion in its init already answers.
func check(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt) {
	value, guard, declares, ok := assertion(pass, ifStmt.Init)
	if !ok {
		return
	}
	cs := conjuncts(ifStmt.Cond)
	guarded := slices.IndexFunc(cs, func(c conjunct) bool {
		return sameObject(pass, c.expr, guard)
	})
	checked := slices.IndexFunc(cs, func(c conjunct) bool {
		return isNotNil(pass, c.expr, value)
	})
	if guarded < 0 || checked < 0 {
		return
	}
	// A check ahead of the guard skips the operands between them when the
	// assertion failed, so it is the guard only where nothing sits between.
	if checked < guarded-1 {
		return
	}
	if rewritten(pass, cs[:checked], value, guard) {
		return
	}
	pos, end := cs[checked].drop()
	read, wrote := usage(pass, file, value, cs[checked].expr)
	// The name has to survive the deletion: something still reads it, or
	// nothing assigns it, so _ can take its place.
	survives := read || !wrote
	var fixes []analysis.SuggestedFix
	if !commented(file.Comments, pos, end) && survives {
		edits := []analysis.TextEdit{{Pos: pos, End: end}}
		for _, p := range cs[checked].groups {
			if members(cs, p) == 2 {
				edits = append(edits,
					analysis.TextEdit{Pos: p.Lparen, End: p.Lparen + 1},
					analysis.TextEdit{Pos: p.Rparen, End: p.Rparen + 1},
				)
			}
		}
		if declares && !read {
			edits = append(edits, analysis.TextEdit{
				Pos:     value.Pos(),
				End:     value.End(),
				NewText: []byte("_"),
			})
		}
		fixes = []analysis.SuggestedFix{{
			Message:   "Drop the nil check",
			TextEdits: edits,
		}}
	}
	msg := value.Name + " is never nil when " + guard.Name + " is true"
	pass.Report(analysis.Diagnostic{
		Pos:            cs[checked].expr.Pos(),
		End:            cs[checked].expr.End(),
		Message:        msg,
		SuggestedFixes: fixes,
	})
}

// conjunct is one operand of a conjunction, with the && that joins it to a
// neighbor, and the groups it sits in.
type conjunct struct {
	expr   ast.Expr
	op     token.Pos
	groups []*ast.ParenExpr
}

// drop returns the span to delete so that the operand and the && joining it to
// its neighbor both go.
func (c conjunct) drop() (pos, end token.Pos) {
	if c.expr.End() <= c.op {
		return c.expr.Pos(), c.op + 2
	}
	return c.op, c.expr.End()
}

// conjuncts returns the operands of expr in source order when expr is a
// conjunction, and expr on its own when it is anything else.
func conjuncts(expr ast.Expr) []conjunct {
	return split(nil, expr, token.NoPos, nil)
}

// split appends the operands of expr, which the operator at op joins, inside
// the groups they sit in.
func split(cs []conjunct, expr ast.Expr, op token.Pos, groups []*ast.ParenExpr) []conjunct {
	if p, isParen := expr.(*ast.ParenExpr); isParen {
		groups = append(slices.Clip(groups), p)
	}
	bin, ok := ast.Unparen(expr).(*ast.BinaryExpr)
	if !ok || bin.Op != token.LAND {
		return append(cs, conjunct{expr, op, groups})
	}
	cs = split(cs, bin.X, bin.OpPos, groups)
	return split(cs, bin.Y, bin.OpPos, groups)
}

// assertion returns the result and guard an if init binds from an assertion
// that cannot yield nil. It also reports whether the init declares them.
func assertion(pass *analysis.Pass, init ast.Stmt) (value, guard *ast.Ident, declares, ok bool) {
	assign, found := init.(*ast.AssignStmt)
	if !found {
		return nil, nil, false, false
	}
	if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return nil, nil, false, false
	}
	assert, found := ast.Unparen(assign.Rhs[0]).(*ast.TypeAssertExpr)
	if !found || assert.Type == nil {
		return nil, nil, false, false
	}
	value, found = assign.Lhs[0].(*ast.Ident)
	if !found {
		return nil, nil, false, false
	}
	guard, found = assign.Lhs[1].(*ast.Ident)
	if !found {
		return nil, nil, false, false
	}
	if !neverNil(pass.TypesInfo.TypeOf(assert.Type)) {
		return nil, nil, false, false
	}
	return value, guard, assign.Tok == token.DEFINE, true
}

// neverNil reports whether an assertion to t cannot yield nil, which is true
// of an interface type: the result carries the dynamic type the source held,
// and a nil interface is one that holds no type at all.
func neverNil(t types.Type) bool {
	if t == nil {
		return false
	}
	// A type parameter can be instantiated with a nil-able type, so the
	// result can be nil.
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// sameObject reports whether e is an identifier that names the same object
// as id.
func sameObject(pass *analysis.Pass, e ast.Expr, id *ast.Ident) bool {
	name, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(name) == pass.TypesInfo.ObjectOf(id)
}

// isNotNil reports whether e compares value with nil as not equal.
func isNotNil(pass *analysis.Pass, e ast.Expr, value *ast.Ident) bool {
	bin, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	if isNil(pass, bin.Y) {
		return sameObject(pass, bin.X, value)
	}
	return isNil(pass, bin.X) && sameObject(pass, bin.Y, value)
}

// rewritten reports whether an operand can give either name a new value,
// through its address or from a function literal that writes it.
func rewritten(pass *analysis.Pass, others []conjunct, value, guard *ast.Ident) bool {
	mutates := func(e ast.Expr) bool {
		return sameObject(pass, e, value) || sameObject(pass, e, guard)
	}
	for _, c := range others {
		var found bool
		ast.Inspect(c.expr, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.UnaryExpr:
				found = found || (n.Op == token.AND && mutates(n.X))
			case *ast.FuncLit:
				for id := range written(n) {
					found = found || mutates(id)
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// written returns the identifiers n assigns.
func written(n ast.Node) map[*ast.Ident]struct{} {
	writes := make(map[*ast.Ident]struct{})
	ast.Inspect(n, func(n ast.Node) bool {
		var lhs []ast.Expr
		switch n := n.(type) {
		case *ast.AssignStmt:
			lhs = n.Lhs
		case *ast.RangeStmt:
			lhs = []ast.Expr{n.Key, n.Value}
		}
		for _, e := range lhs {
			if id, ok := ast.Unparen(e).(*ast.Ident); ok {
				writes[id] = struct{}{}
			}
		}
		return true
	})
	return writes
}

// usage reports what the file does with value once the check goes: whether
// anything outside dropped reads it, and whether anything assigns it.
func usage(pass *analysis.Pass, file *ast.File, value *ast.Ident, dropped ast.Expr) (read, wrote bool) {
	var (
		obj    = pass.TypesInfo.ObjectOf(value)
		writes = written(file)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || pass.TypesInfo.ObjectOf(id) != obj {
			return true
		}
		// A definition names the object without reading it.
		if pass.TypesInfo.Defs[id] == obj {
			return true
		}
		if _, ok := writes[id]; ok {
			wrote = true
			return true
		}
		// The check is on its way out, so what it reads does not count.
		if id.Pos() >= dropped.Pos() && id.End() <= dropped.End() {
			return true
		}
		read = true
		return true
	})
	return read, wrote
}

// members returns how many of cs sit inside the group p.
func members(cs []conjunct, p *ast.ParenExpr) (n int) {
	for _, c := range cs {
		if c.expr.Pos() > p.Lparen && c.expr.End() <= p.Rparen {
			n++
		}
	}
	return n
}

// commented reports whether a comment sits in the span a fix would delete.
func commented(comments []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}

// isNil reports whether e is the predeclared nil.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("nil")
}
