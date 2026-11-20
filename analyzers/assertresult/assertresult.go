// Package assertresult provides an analysis.Analyzer that reports a case that
// asserts the value its type switch already tested.
//
// A type switch compares the dynamic type of the value its guard tests
// against each case (https://go.dev/ref/spec#Type_switches). A case that lists
// one type is entered only when the type matches, so an assertion of the value
// at that type inside the case repeats a test that cannot fail.
//
//	switch x.(type) {
//	case int:
//		print(x.(int))
//	}
//
// The fix reads the value the switch binds in place of each assertion, binding
// a name in the guard when the guard names none. It is withheld where an edit
// would delete a comment, and the README catalogs the forms it leaves alone.
//
// The rule is adapted from staticcheck's S1034, "Use result of type assertion
// to simplify cases".
package assertresult

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "assertresult",
	Doc:  "reports a case that asserts the value its type switch already tested",
	Run:  run,
}

// guard is what a type switch statement guards, and what it binds.
type guard struct {
	assert *ast.TypeAssertExpr
	value  ast.Expr
	name   *ast.Ident // the bound name, nil when the guard binds nothing
}

// site is one type switch under inspection.
type site struct {
	stmt  *ast.TypeSwitchStmt
	fn    ast.Node
	guard guard
}

// finding is one switch to report and the assertions the fix replaces.
type finding struct {
	stmt      *ast.TypeSwitchStmt
	guard     guard
	offenders []*ast.TypeAssertExpr
}

// checker holds what one file shares across its type switches.
type checker struct {
	pass    *analysis.Pass
	file    *ast.File
	parents map[ast.Node]ast.Node
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			c      = checker{pass, file, astexpr.Parents(file)}
			found  = c.findings()
			chosen = bindings(found)
		)
		for _, f := range found {
			c.report(f, chosen[f.stmt])
		}
	}
	return nil, nil
}

// findings returns the switches in the file that the rule reports, in source
// order.
func (c checker) findings() (out []finding) {
	ast.Inspect(c.file, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.TypeSwitchStmt); ok {
			if f, ok := c.finding(stmt); ok {
				out = append(out, f)
			}
		}
		return true
	})
	return out
}

// finding returns what the rule reports about stmt, if anything.
func (c checker) finding(stmt *ast.TypeSwitchStmt) (finding, bool) {
	g, ok := guardOf(stmt)
	if !ok {
		return finding{}, false
	}
	at := site{stmt, enclosing(c.parents, stmt), g}
	var offenders []*ast.TypeAssertExpr
	for _, clause := range stmt.Body.List {
		clause, ok := clause.(*ast.CaseClause)
		if !ok || len(clause.List) != 1 {
			continue
		}
		typ := c.pass.TypesInfo.TypeOf(clause.List[0])
		if typ == nil {
			continue
		}
		ast.Inspect(clause, func(n ast.Node) bool {
			again, ok := n.(*ast.TypeAssertExpr)
			if !ok || c.skip(again, clause, at) {
				return true
			}
			if c.replaceable(again, g.value, typ) {
				offenders = append(offenders, again)
			}
			return true
		})
	}
	if len(offenders) == 0 {
		return finding{}, false
	}
	return finding{stmt, g, offenders}, true
}

// skip reports whether the fix cannot stand in for assert: it runs outside the
// case, another switch reports it, or a write changes the value it tests.
func (c checker) skip(assert *ast.TypeAssertExpr, clause *ast.CaseClause, at site) bool {
	if enclosed(c.parents, clause, assert) || c.claimed(clause, assert, at) {
		return true
	}
	obj := c.pass.TypesInfo.ObjectOf(ident(at.guard.value))
	// An assertion a loop re-runs tests whatever the loop left behind, so
	// there a write after it still reaches it.
	pos := assert.Pos()
	if repeated(c.parents, clause, assert) {
		pos = clause.End()
	}
	if written(c.pass, clause, obj, pos) {
		return true
	}
	return indirect(c.pass, at.fn, obj, pos)
}

// claimed reports whether a type switch between root and node reports node
// itself, which a switch guarding the same value does.
func (c checker) claimed(root, node ast.Node, at site) bool {
	for n := c.parents[node]; n != nil && n != root; n = c.parents[n] {
		stmt, ok := n.(*ast.TypeSwitchStmt)
		if !ok {
			continue
		}
		g, ok := guardOf(stmt)
		if ok && astexpr.Same(c.pass, g.value, at.guard.value) {
			return true
		}
	}
	return false
}

// repeated reports whether assert runs more than once, as one inside a loop
// does, or after a goto that jumps back to it.
func repeated(parents map[ast.Node]ast.Node, clause *ast.CaseClause, assert *ast.TypeAssertExpr) bool {
	for n := parents[assert]; n != nil && n != clause; n = parents[n] {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	var found bool
	ast.Inspect(clause, func(n ast.Node) bool {
		if branch, ok := n.(*ast.BranchStmt); ok && branch.Tok == token.GOTO {
			found = true
		}
		return !found
	})
	return found
}

// report writes the diagnostic for f, with the fix that reads the switch's
// value in place of each assertion.
func (c checker) report(f finding, bound string) {
	var (
		operand = ident(f.guard.value).Name
		message = "a case asserts the value again; bind it in the switch"
		fix     = "Write " + bound + " := " + operand + ".(type)"
	)
	if name := f.guard.name; name != nil {
		bound = name.Name
		fix = "Read " + bound + " from the switch"
		message = "a case asserts the value again; read " + bound +
			" from the switch"
	}
	diagnostic := analysis.Diagnostic{
		Pos:     f.stmt.Switch,
		End:     f.guard.assert.End(),
		Message: message,
	}
	if edits, safe := c.rewrite(f, bound); safe {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   fix,
			TextEdits: edits,
		}}
	}
	c.pass.Report(diagnostic)
}

// rewrite returns the edits that read the switch's value in place of each
// assertion, and reports whether no edit deletes a comment.
func (c checker) rewrite(f finding, bound string) ([]analysis.TextEdit, bool) {
	var edits []analysis.TextEdit
	if f.guard.name == nil {
		if commented(c.file, f.guard.assert) {
			return nil, false
		}
		operand := ident(f.guard.value).Name
		edits = append(edits, analysis.TextEdit{
			Pos:     f.guard.assert.Pos(),
			End:     f.guard.assert.End(),
			NewText: []byte(bound + " := " + operand + ".(type)"),
		})
	}
	for _, offender := range f.offenders {
		if commented(c.file, offender) {
			return nil, false
		}
		edits = append(edits, analysis.TextEdit{
			Pos:     offender.Pos(),
			End:     offender.End(),
			NewText: []byte(bound),
		})
	}
	return edits, true
}

// commented reports whether a comment sits inside node, which an edit over
// node's text would delete.
func commented(file *ast.File, node ast.Node) bool {
	return astexpr.Commented(file, node.Pos(), node.End())
}

// guardOf returns what stmt guards and what it binds.
func guardOf(stmt *ast.TypeSwitchStmt) (guard, bool) {
	var (
		bound *ast.Ident
		expr  ast.Expr
	)
	switch assign := stmt.Assign.(type) {
	case *ast.ExprStmt:
		expr = assign.X
	case *ast.AssignStmt:
		if assign.Tok != token.DEFINE || len(assign.Lhs)*len(assign.Rhs) != 1 {
			return guard{}, false
		}
		var ok bool
		if bound, ok = assign.Lhs[0].(*ast.Ident); !ok {
			return guard{}, false
		}
		expr = assign.Rhs[0]
	default:
		return guard{}, false
	}
	assert, ok := ast.Unparen(expr).(*ast.TypeAssertExpr)
	if !ok || assert.Type != nil || ident(assert.X) == nil {
		return guard{}, false
	}
	return guard{assert: assert, value: assert.X, name: bound}, true
}

// ident returns e as an identifier, or nil when it is anything else.
func ident(e ast.Expr) *ast.Ident {
	id, _ := ast.Unparen(e).(*ast.Ident)
	return id
}

// replaceable reports whether the fix can put the switch's value in place of
// assert.
func (c checker) replaceable(assert *ast.TypeAssertExpr, value ast.Expr, typ types.Type) bool {
	if assert.Type == nil || !astexpr.Same(c.pass, assert.X, value) {
		return false
	}
	if !types.Identical(c.pass.TypesInfo.TypeOf(assert.Type), typ) {
		return false
	}
	return !commaOk(c.parents, assert)
}

// commaOk reports whether assert stands for the value and the success result,
// as in v, ok := x.(T), where one value cannot take its place.
func commaOk(parents map[ast.Node]ast.Node, assert *ast.TypeAssertExpr) bool {
	for node := parents[assert]; node != nil; node = parents[node] {
		switch n := node.(type) {
		case *ast.ParenExpr:
			continue
		case *ast.AssignStmt:
			return len(n.Lhs) == 2 && len(n.Rhs) == 1 &&
				ast.Unparen(n.Rhs[0]) == ast.Expr(assert)
		case *ast.ValueSpec:
			return len(n.Names) == 2 && len(n.Values) == 1 &&
				ast.Unparen(n.Values[0]) == ast.Expr(assert)
		default:
			return false
		}
	}
	return false
}

// written reports whether an assignment before pos in node can change obj,
// which leaves a later assertion testing a value the switch did not.
func written(pass *analysis.Pass, node ast.Node, obj types.Object, pos token.Pos) bool {
	return assigns(pass, node, obj, func(target ast.Expr) bool {
		return target.Pos() < pos
	})
}

// indirect reports whether a write can reach obj before pos without passing
// through the case: an address of obj is taken, or a function literal defined
// before pos assigns it and a call runs that.
func indirect(pass *analysis.Pass, node ast.Node, obj types.Object, pos token.Pos) bool {
	if node == nil {
		return false
	}
	if aliased(pass, node, obj, pos) {
		return true
	}
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		if lit, ok := n.(*ast.FuncLit); ok {
			found = lit.Pos() < pos && assigns(pass, lit, obj, anyPosition)
			return false
		}
		return true
	})
	return found
}

// aliased reports whether an address of obj is taken before pos.
func aliased(pass *analysis.Pass, node ast.Node, obj types.Object, pos token.Pos) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		unary, ok := n.(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND || unary.Pos() >= pos {
			return true
		}
		if id := ident(unary.X); names(pass, id, obj) {
			found = true
			return false
		}
		return true
	})
	return found
}

// names reports whether id names obj.
func names(pass *analysis.Pass, id *ast.Ident, obj types.Object) bool {
	return id != nil && pass.TypesInfo.ObjectOf(id) == obj
}

// anyPosition keeps every target.
func anyPosition(ast.Expr) bool { return true }

// assigns reports whether an assignment in node names obj as a target that
// keep accepts.
func assigns(pass *analysis.Pass, node ast.Node, obj types.Object, keep func(ast.Expr) bool) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		var targets []ast.Expr
		switch n := n.(type) {
		case *ast.AssignStmt:
			targets = n.Lhs
		case *ast.RangeStmt:
			targets = []ast.Expr{n.Key, n.Value}
		}
		for _, target := range targets {
			if target == nil || !keep(target) {
				continue
			}
			if names(pass, ident(target), obj) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// enclosed reports whether node sits inside a function literal within root,
// where it runs at a time the case's statements do not fix.
func enclosed(parents map[ast.Node]ast.Node, root, node ast.Node) bool {
	for n := parents[node]; n != nil && n != root; n = parents[n] {
		if _, ok := n.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

// enclosing returns the function node sits in.
func enclosing(parents map[ast.Node]ast.Node, node ast.Node) ast.Node {
	for node := parents[node]; node != nil; node = parents[node] {
		switch node.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return node
		}
	}
	return nil
}

// bindings returns the name the fix binds for each reported switch that binds
// nothing, keyed by the switch.
//
// A binding shadows what the cases read, so a switch avoids the names its own
// body uses, and it avoids the names chosen for the reported switches nested
// in it, whose bodies would otherwise read the outer binding.
func bindings(found []finding) map[*ast.TypeSwitchStmt]string {
	out := make(map[*ast.TypeSwitchStmt]string, len(found))
	// A nested switch starts later in the file, so its name is known by the
	// time the switch around it is named.
	for i, f := range slices.Backward(found) {
		stmt := f.stmt
		if f.guard.name != nil {
			continue
		}
		used := identifiers(stmt)
		for _, inner := range found[i+1:] {
			name, ok := out[inner.stmt]
			if ok && inner.stmt.End() < stmt.End() {
				used[name] = struct{}{}
			}
		}
		out[stmt] = free(used)
	}
	return out
}

// identifiers returns the names every identifier in node carries.
func identifiers(node ast.Node) map[string]struct{} {
	used := make(map[string]struct{})
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = struct{}{}
		}
		return true
	})
	return used
}

// free returns the first of v, v1, v2, and so on that used does not hold.
func free(used map[string]struct{}) string {
	name := "v"
	for i := 1; ; i++ {
		if _, ok := used[name]; !ok {
			return name
		}
		name = "v" + strconv.Itoa(i)
	}
}
