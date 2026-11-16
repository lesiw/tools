// Package resetvalue provides an analysis.Analyzer that reports a read of the
// value returned by (*time.Timer).Reset that decides whether to drain the
// timer's channel.
//
// Timer.Reset's documentation says it "returns true if the timer had been
// active, false if the timer had expired or been stopped"
// (https://pkg.go.dev/time#Timer.Reset). Before Go 1.23 the channel was
// buffered, so an expired timer's value could still be received, and the
// result was read to decide whether to drain it. Draining after Reset races
// with the new timer, which can send before the receive runs.
//
// Since Go 1.23 a chan-based timer created with NewTimer has an unbuffered
// channel; the same documentation says "any receive from t.C after Reset has
// returned is guaranteed not to receive a time value corresponding to the
// previous timer settings". A drain after Reset therefore receives the new
// timer's value or blocks, so reading the result to choose the branch is
// wrong at every Go version.
//
// The rule matches the branch that runs when Reset returns false, when that
// branch receives from the timer's channel. The condition is a Reset call,
// with or without a negation, or a local assigned by Reset and tested by the
// next statement, by the statement before a labeled if, or by the if's init.
// The branch that runs when Reset returns true is a wait, and it is left
// alone. A bare call, an assignment to the blank identifier, a result used
// for another purpose, a receive from another channel, a receive inside a
// function literal, a method expression, a condition that combines the
// result with another expression, and a switch or loop header are left
// alone. The rule follows assignments lexically, so an assignment to it, or a
// Reset or Stop call on it, before the receive stops the search. A call in a
// test file is left alone,
// because a test may assert the documented result. The README catalogs
// every check, exemption, and fix.
//
// At Go 1.27 and later, or when the file names no version, the fix replaces an
// if whose condition is the negated Reset call, whose body is only the
// receive, and which has no init or else and is not an else-if. It is
// withheld where a comment sits in the if. Go 1.23 through 1.26
// accept a godebug line of asynctimerchan=1, which restores the buffered
// channel (https://go.dev/doc/godebug#go-123), so the rule reports without a
// fix there. Below Go 1.23 the safe form is to stop and drain before Reset,
// and the rule reports without a fix.
//
// The rule is adapted from staticcheck's SA1025, which reports a Reset result
// that gates a receive from any channel of type <-chan time.Time. This rule
// requires the receive and the Reset call to name the same timer and can
// delete the branch.
package resetvalue

import (
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "resetvalue",
	Doc:  "reports a Reset result that decides whether to drain the timer",
	Run:  run,
}

// site is an if statement and the statement list that holds it. The list is
// nil for an else-if, which is not written in a statement list.
type site struct {
	stmt   *ast.IfStmt
	list   []ast.Stmt
	index  int
	elseIf bool
}

// resetRead is a Reset call, the timer it resets, and whether the condition
// that tests it is negated.
type resetRead struct {
	call    *ast.CallExpr
	recv    ast.Expr
	method  *types.Selection
	field   *types.Var
	negated bool
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if isTest(pass, file) {
			continue
		}
		for _, s := range sites(file) {
			check(pass, file, s)
		}
	}
	return nil, nil
}

// sites returns every if statement in file, with the list that holds the ones
// written directly in a block or clause body.
func sites(file *ast.File) []*site {
	var (
		byIf  = make(map[*ast.IfStmt]*site)
		order []*site
	)
	ast.Inspect(file, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			s := &site{stmt: ifStmt}
			byIf[ifStmt] = s
			order = append(order, s)
		}
		return true
	})
	fill := func(list []ast.Stmt) {
		for i, stmt := range list {
			ifStmt, ok := stmt.(*ast.IfStmt)
			if !ok {
				if labeled, ok := stmt.(*ast.LabeledStmt); ok {
					ifStmt, _ = labeled.Stmt.(*ast.IfStmt)
				}
			}
			if ifStmt != nil {
				byIf[ifStmt].list, byIf[ifStmt].index = list, i
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BlockStmt:
			fill(n.List)
		case *ast.CaseClause:
			fill(n.Body)
		case *ast.CommClause:
			fill(n.Body)
		}
		return true
	})
	for _, s := range order {
		if elseIf, ok := s.stmt.Else.(*ast.IfStmt); ok {
			byIf[elseIf].elseIf = true
		}
	}
	return order
}

// check reports s when the branch that runs after Reset returns false receives
// from the timer's channel.
func check(pass *analysis.Pass, file *ast.File, s *site) {
	r, ok := resetCall(pass, s)
	if !ok || r.recv == nil {
		return
	}
	falseBranch := s.stmt.Else
	if r.negated {
		falseBranch = s.stmt.Body
	}
	if _, ok := falseBranch.(*ast.IfStmt); ok {
		return
	}
	var extra ast.Node
	if s.stmt.Init != nil && r.call.Pos() < s.stmt.Pos() {
		extra = s.stmt.Init
	}
	if !drains(pass, falseBranch, r, extra) {
		return
	}
	message := "a receive after Reset can take the new timer's value"
	if before123(pass, file) {
		message = "a receive after Reset can take a stale or new timer value"
	}
	diagnostic := analysis.Diagnostic{
		Pos:     r.call.Pos(),
		End:     r.call.End(),
		Message: message,
	}
	if fixable(pass, file, s, r) {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Drop the drain",
			TextEdits: []analysis.TextEdit{{
				Pos:     s.stmt.Pos(),
				End:     s.stmt.End(),
				NewText: []byte(astexpr.Text(r.call)),
			}},
		}}
	}
	pass.Report(diagnostic)
}

// resetCall returns the Reset call that s tests.
func resetCall(pass *analysis.Pass, s *site) (resetRead, bool) {
	var (
		cond    = ast.Unparen(s.stmt.Cond)
		negated bool
	)
	if unary, ok := cond.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
		negated = true
		cond = ast.Unparen(unary.X)
	}
	if call, ok := cond.(*ast.CallExpr); ok {
		if recv, field, method, ok := timerReset(pass, call); ok {
			return resetRead{call, recv, method, field, negated}, true
		}
	}
	id, ok := cond.(*ast.Ident)
	if !ok {
		return resetRead{}, false
	}
	obj := pass.TypesInfo.ObjectOf(id)
	if obj == nil {
		return resetRead{}, false
	}
	if s.stmt.Init != nil {
		if call, ok := assigned(pass, s.stmt.Init, obj); ok {
			recv, field, method, ok := timerReset(pass, call)
			if !ok {
				return resetRead{}, false
			}
			return resetRead{call, recv, method, field, negated}, true
		}
		if mentions(pass, s.stmt.Init, obj) {
			return resetRead{}, false
		}
	}
	if s.list != nil && s.index > 0 {
		if call, ok := assigned(pass, s.list[s.index-1], obj); ok {
			recv, field, method, ok := timerReset(pass, call)
			if !ok {
				return resetRead{}, false
			}
			return resetRead{call, recv, method, field, negated}, true
		}
	}
	return resetRead{}, false
}

// assigned returns the Reset call that stmt assigns to obj.
func assigned(pass *analysis.Pass, stmt ast.Stmt, obj types.Object) (*ast.CallExpr, bool) {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		if stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE {
			return nil, false
		}
		return assignedPair(pass, stmt.Lhs, stmt.Rhs, obj)
	case *ast.DeclStmt:
		gen, ok := stmt.Decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			return nil, false
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			lhs := make([]ast.Expr, len(vs.Names))
			for i, name := range vs.Names {
				lhs[i] = name
			}
			if call, ok := assignedPair(pass, lhs, vs.Values, obj); ok {
				return call, true
			}
		}
	}
	return nil, false
}

// assignedPair returns the Reset call that assigns obj from the matching
// expression in rhs.
func assignedPair(pass *analysis.Pass, lhs, rhs []ast.Expr, obj types.Object) (*ast.CallExpr, bool) {
	if len(lhs) != len(rhs) {
		return nil, false
	}
	last := -1
	for i, left := range lhs {
		id, ok := ast.Unparen(left).(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(id) == obj {
			last = i
		}
	}
	if last < 0 {
		return nil, false
	}
	call, ok := ast.Unparen(rhs[last]).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	recv, _, method, ok := timerReset(pass, call)
	if !ok {
		return nil, false
	}
	for j, other := range lhs {
		if j == last {
			continue
		}
		if astexpr.Mentions(pass, recv, other) {
			return nil, false
		}
		if astexpr.Mentions(pass, other, recv) {
			return nil, false
		}
	}
	for j, other := range rhs {
		if j != last && callsIn(pass, other, recv, method) {
			return nil, false
		}
	}
	return call, true
}

// mentions reports whether n names obj anywhere.
func mentions(pass *analysis.Pass, n ast.Node, obj types.Object) bool {
	var found bool
	ast.Inspect(n, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(id) == obj {
			found = true
		}
		return !found
	})
	return found
}

// timerReset returns the receiver of call and time.Timer.C when call is the
// (*time.Timer).Reset method.
func timerReset(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, *types.Var, *types.Selection, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil, nil, false
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection != nil && selection.Kind() == types.MethodExpr {
		return nil, nil, nil, false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok {
		return nil, nil, nil, false
	}
	if fn.Pkg() == nil || fn.Pkg().Path() != "time" || fn.Name() != "Reset" {
		return nil, nil, nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || !isTimer(sig.Recv().Type()) {
		return nil, nil, nil, false
	}
	obj, _, _ := types.LookupFieldOrMethod(sig.Recv().Type(), true, nil, "C")
	field, ok := obj.(*types.Var)
	if !ok {
		return nil, nil, nil, false
	}
	return sel.X, field, selection, true
}

// timerStop returns the receiver of call and its selection when call is the
// (*time.Timer).Stop method.
func timerStop(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, *types.Selection, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil, false
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection != nil && selection.Kind() == types.MethodExpr {
		return nil, nil, false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok {
		return nil, nil, false
	}
	if fn.Pkg() == nil || fn.Pkg().Path() != "time" || fn.Name() != "Stop" {
		return nil, nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || !isTimer(sig.Recv().Type()) {
		return nil, nil, false
	}
	return sel.X, selection, true
}

// isTimer reports whether t is *time.Timer.
func isTimer(t types.Type) bool {
	ptr, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "time" &&
		obj.Name() == "Timer"
}

// drains reports whether n receives from the channel of recv.
func drains(pass *analysis.Pass, n ast.Node, r resetRead, extra ast.Node) bool {
	if n == nil {
		return false
	}
	var found bool
	ast.Inspect(n, func(node ast.Node) bool {
		if found {
			return false
		}
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.UnaryExpr:
			if node.Op != token.ARROW {
				return true
			}
			sel, ok := ast.Unparen(node.X).(*ast.SelectorExpr)
			if !ok || !timerField(pass, sel, r.field) {
				return true
			}
			field := pass.TypesInfo.Selections[sel]
			if !sameTimer(pass, r.recv, sel.X, r.method, field) {
				return true
			}
			if rewritten(pass, n, r, node.Pos()) {
				return true
			}
			if extra != nil && rewritten(pass, extra, r, node.Pos()) {
				return true
			}
			found = true
			return false
		}
		return true
	})
	return found
}

// rewritten reports whether n assigns the timer, or a value named in the
// expression that selects it, before the receive at before. A Reset or Stop
// call on the timer counts too.
func rewritten(pass *analysis.Pass, n ast.Node, r resetRead, before token.Pos) bool {
	var (
		parts            = receiverParts(r.recv)
		base, recvFields = timerPath(pass, r.recv)
		fields           = append(selectionPath(r.method), recvFields...)
	)
	if len(parts) == 0 {
		return false
	}
	var found bool
	ast.Inspect(n, func(node ast.Node) bool {
		if found {
			return false
		}
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if node.Pos() < before {
				if writes(pass, node.Lhs, parts, fields, r.field, base) {
					found = true
				}
			}
		case *ast.IncDecStmt:
			if node.Pos() < before {
				lhs := []ast.Expr{node.X}
				if writes(pass, lhs, parts, fields, r.field, base) {
					found = true
				}
			}
		case *ast.RangeStmt:
			if node.Pos() < before {
				lhs := []ast.Expr{node.Key, node.Value}
				if writes(pass, lhs, parts, fields, r.field, base) {
					found = true
				}
			}
		case *ast.CallExpr:
			if node != r.call && node.Pos() < before {
				if callsTimer(pass, node, r) {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// callsTimer reports whether call resets or stops the timer r names.
func callsTimer(pass *analysis.Pass, call *ast.CallExpr, r resetRead) bool {
	return callsTimerWith(pass, call, r.recv, r.method)
}

// callsTimerWith reports whether call resets or stops the timer that recv
// names.
func callsTimerWith(pass *analysis.Pass, call *ast.CallExpr, recv ast.Expr, method *types.Selection) bool {
	if r, _, m, ok := timerReset(pass, call); ok {
		return sameTimer(pass, recv, r, method, m)
	}
	if r, m, ok := timerStop(pass, call); ok {
		return sameTimer(pass, recv, r, method, m)
	}
	return false
}

// callsIn reports whether e calls Reset or Stop on the timer that recv names.
func callsIn(pass *analysis.Pass, e, recv ast.Expr, method *types.Selection) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if callsTimerWith(pass, call, recv, method) {
				found = true
			}
		}
		return !found
	})
	return found
}

// writes reports whether any of lhs assigns the timer or a value the timer
// expression reads.
func writes(pass *analysis.Pass, lhs, parts []ast.Expr, fields []*types.Var, field *types.Var, base ast.Expr) bool {
	for _, e := range lhs {
		if e == nil {
			continue
		}
		if sel, ok := ast.Unparen(e).(*ast.SelectorExpr); ok {
			obj, _ := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
			if obj == field || slices.Contains(fields, obj) {
				if onPath(pass, sel.X, base, parts, fields) {
					return true
				}
			}
		}
		if star, ok := ast.Unparen(e).(*ast.StarExpr); ok {
			if onPath(pass, star.X, base, parts, fields) {
				return true
			}
		}
		if part(pass, e, parts) {
			return true
		}
	}
	return false
}

// onPath reports whether e is the timer base, one of its parts, or a selector
// on the embedded path to it.
func onPath(pass *analysis.Pass, e, base ast.Expr, parts []ast.Expr, fields []*types.Var) bool {
	if astexpr.Same(pass, e, base) || part(pass, e, parts) {
		return true
	}
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	obj, _ := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	if obj == nil || !slices.Contains(fields, obj) {
		return false
	}
	return onPath(pass, sel.X, base, parts, fields)
}

// part reports whether e is one of parts.
func part(pass *analysis.Pass, e ast.Expr, parts []ast.Expr) bool {
	for _, p := range parts {
		if astexpr.Same(pass, e, p) {
			return true
		}
	}
	return false
}

// receiverParts returns the receiver expression and the expressions that
// select it: the prefixes of a selector chain and the index of an index
// expression.
func receiverParts(recv ast.Expr) []ast.Expr {
	var (
		parts []ast.Expr
		walk  func(ast.Expr)
	)
	walk = func(e ast.Expr) {
		e = ast.Unparen(e)
		parts = append(parts, e)
		switch x := e.(type) {
		case *ast.SelectorExpr:
			walk(x.X)
		case *ast.IndexExpr:
			ast.Inspect(x.Index, func(n ast.Node) bool {
				if e, ok := n.(ast.Expr); ok {
					parts = append(parts, e)
				}
				return true
			})
			walk(x.X)
		case *ast.StarExpr:
			walk(x.X)
		}
	}
	walk(recv)
	return parts
}

// sameTimer reports whether x names the timer that recv does, including a
// timer reached through embedded fields on either side.
func sameTimer(pass *analysis.Pass, recv, x ast.Expr, method, field *types.Selection) bool {
	if method == nil || field == nil {
		return astexpr.Same(pass, recv, x)
	}
	var (
		recvBase, recvPath = timerPath(pass, recv)
		xBase, xPath       = timerPath(pass, x)
	)
	if !astexpr.Same(pass, recvBase, xBase) {
		return false
	}
	recvPath = append(recvPath, selectionPath(method)...)
	xPath = append(xPath, selectionPath(field)...)
	if len(recvPath) != len(xPath) {
		return false
	}
	for i, f := range recvPath {
		if f != xPath[i] {
			return false
		}
	}
	return true
}

// timerPath returns the base expression and the embedded fields that lead to
// e.
func timerPath(pass *analysis.Pass, e ast.Expr) (ast.Expr, []*types.Var) {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return e, nil
	}
	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	if !ok || !obj.Embedded() {
		return e, nil
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection == nil {
		return e, nil
	}
	base, path := timerPath(pass, sel.X)
	path = append(path, selectionPath(selection)...)
	return base, append(path, obj)
}

// selectionPath returns the embedded fields in a selection's index, without
// the method or field the selection names.
func selectionPath(sel *types.Selection) (out []*types.Var) {
	index := sel.Index()
	if len(index) == 0 {
		return nil
	}
	t := types.Unalias(sel.Recv())
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	for _, i := range index[:len(index)-1] {
		named, ok := t.(*types.Named)
		if !ok {
			return nil
		}
		str, ok := named.Underlying().(*types.Struct)
		if !ok || i < 0 || i >= str.NumFields() {
			return nil
		}
		f := str.Field(i)
		out = append(out, f)
		t = types.Unalias(f.Type())
		if ptr, ok := t.(*types.Pointer); ok {
			t = types.Unalias(ptr.Elem())
		}
	}
	return out
}

// fixable reports whether the if that reads the value can be replaced by the
// Reset call.
func fixable(pass *analysis.Pass, file *ast.File, s *site, r resetRead) bool {
	if !r.negated {
		return false
	}
	if s.elseIf {
		return false
	}
	if s.stmt.Init != nil || s.stmt.Else != nil {
		return false
	}
	if !condition(s.stmt.Cond, r.call) {
		return false
	}
	if !onlyReceive(pass, s.stmt.Body, r) {
		return false
	}
	if !modern(pass, file) {
		return false
	}
	return !commented(file, s.stmt.Pos(), s.stmt.End())
}

// commented reports whether a comment is between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// condition reports whether cond, with or without a negation, is call.
func condition(cond ast.Expr, call *ast.CallExpr) bool {
	cond = ast.Unparen(cond)
	if unary, ok := cond.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
		cond = ast.Unparen(unary.X)
	}
	return cond == call
}

// onlyReceive reports whether block holds only a receive from recv's channel.
func onlyReceive(pass *analysis.Pass, block *ast.BlockStmt, r resetRead) bool {
	if block == nil || len(block.List) != 1 {
		return false
	}
	stmt, ok := block.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	unary, ok := ast.Unparen(stmt.X).(*ast.UnaryExpr)
	if !ok || unary.Op != token.ARROW {
		return false
	}
	sel, ok := ast.Unparen(unary.X).(*ast.SelectorExpr)
	if !ok || !timerField(pass, sel, r.field) {
		return false
	}
	field := pass.TypesInfo.Selections[sel]
	return sameTimer(pass, r.recv, sel.X, r.method, field)
}

// timerField reports whether sel names the time.Timer.C field.
func timerField(pass *analysis.Pass, sel *ast.SelectorExpr, field *types.Var) bool {
	obj, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Var)
	return ok && obj == field
}

// before123 reports whether the file is compiled before Go 1.23, where the
// timer channel was buffered.
func before123(pass *analysis.Pass, file *ast.File) bool {
	v := pass.TypesInfo.FileVersions[file]
	return v != "" && version.Compare(v, "go1.23") < 0
}

// modern reports whether the file is compiled at Go 1.27 or later, where the
// buffered channel cannot be restored. Go 1.23 through 1.26 accept a godebug
// line of asynctimerchan=1, so a fix there could delete a drain the program
// still needs. An empty version means the type checker was given none, which
// is treated as modern.
func modern(pass *analysis.Pass, file *ast.File) bool {
	v := pass.TypesInfo.FileVersions[file]
	return v == "" || version.Compare(v, "go1.27") >= 0
}

// isTest reports whether file is a test file, which may assert the documented
// result.
func isTest(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(
		pass.Fset.Position(file.Pos()).Filename,
		"_test.go",
	)
}
