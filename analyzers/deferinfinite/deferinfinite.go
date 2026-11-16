// Package deferinfinite provides an analysis.Analyzer that reports a deferred
// call in a loop that cannot end.
//
// The For statements section of the spec says a for statement with no
// condition is equivalent to one whose condition is true
// (https://go.dev/ref/spec#For_statements). The Defer statements section
// says a deferred call runs when the surrounding function returns, including
// when it panics (https://go.dev/ref/spec#Defer_statements). A call in a loop
// that cannot end does not run before the function returns, and each pass adds
// another call to the function's list.
//
//	func f() {
//		for {
//			defer release()
//		}
//	}
//
// The README lists the cases that keep a defer from being reported and the
// loops that are not checked.
//
// Report only: moving the call out of the loop or running it plainly changes
// when it runs, and either may be the repair the author wants.
//
// The rule is adapted from staticcheck's SA5003.
package deferinfinite

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name:     "deferinfinite",
	Doc:      "reports a deferred call in a loop that cannot end",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		owners   = breakOwners(pass)
		reported = make(map[*ast.DeferStmt]bool)
	)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					checkBody(pass, owners, n.Body, reported)
				}
			case *ast.FuncLit:
				checkBody(pass, owners, n.Body, reported)
			}
			return true
		})
	}
	return nil, nil
}

// breakOwners maps each unlabeled break or continue to the statement it leaves
// or repeats.
func breakOwners(pass *analysis.Pass) map[*ast.BranchStmt]ast.Node {
	var (
		insp   = pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
		owners = make(map[*ast.BranchStmt]ast.Node)
		filter = []ast.Node{(*ast.BranchStmt)(nil)}
	)
	insp.WithStack(filter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		stmt := n.(*ast.BranchStmt)
		if stmt.Label != nil {
			return true
		}
		switch stmt.Tok {
		case token.BREAK, token.CONTINUE:
		default:
			return true
		}
		for i := len(stack) - 2; i >= 0; i-- {
			switch stack[i].(type) {
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
				*ast.TypeSwitchStmt, *ast.SelectStmt:
				owners[stmt] = stack[i]
				return true
			}
		}
		return true
	})
	return owners
}

// checkBody checks the loops in body, checking a function literal body in a
// separate call.
func checkBody(pass *analysis.Pass, owners map[*ast.BranchStmt]ast.Node, body *ast.BlockStmt, reported map[*ast.DeferStmt]bool) {
	labels := labelIndex(body)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ForStmt:
			if endless(pass, n) {
				checkLoop(pass, owners, body, n, labels, reported)
			}
		}
		return true
	})
}

// labelIndex returns the labels in body, skipping the labels of a function
// literal.
func labelIndex(body *ast.BlockStmt) map[string]*ast.LabeledStmt {
	labels := make(map[string]*ast.LabeledStmt)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n
		}
		return true
	})
	return labels
}

// endless reports whether loop has no condition or a condition that is a
// constant true.
func endless(pass *analysis.Pass, loop *ast.ForStmt) bool {
	if loop.Cond == nil {
		return true
	}
	tv, ok := pass.TypesInfo.Types[loop.Cond]
	return ok && tv.Value != nil && tv.Value.Kind() == constant.Bool &&
		constant.BoolVal(tv.Value)
}

// checkLoop reports the defers in loop when nothing ends the loop or the
// function.
func checkLoop(pass *analysis.Pass, owners map[*ast.BranchStmt]ast.Node, body *ast.BlockStmt, loop *ast.ForStmt, labels map[string]*ast.LabeledStmt, reported map[*ast.DeferStmt]bool) {
	w := walker{
		pass: pass, loop: loop, labels: labels, body: body, owners: owners,
	}
	w.stmt(loop.Body, 0)
	if w.exits || unwinds(pass, loop) {
		return
	}
	for _, stmt := range w.defers {
		if reported[stmt] {
			continue
		}
		reported[stmt] = true
		pass.Reportf(
			stmt.Pos(),
			"deferred call never runs because an enclosing loop cannot end",
		)
	}
}

// unwinds reports whether loop has a direct panic or runtime.Goexit call that
// runs the function's deferred calls.
func unwinds(pass *analysis.Pass, loop *ast.ForStmt) bool {
	s := &scanner{
		pass:    pass,
		invoked: make(map[*ast.FuncLit]bool),
		skip:    make(map[*ast.FuncLit]bool),
	}
	s.prepare(loop)
	return s.exits(loop.Body) || s.exits(loop.Post)
}

// scanner looks for calls that run the function's deferred calls.
type scanner struct {
	pass    *analysis.Pass
	invoked map[*ast.FuncLit]bool
	skip    map[*ast.FuncLit]bool
}

// prepare records the function literals in node that run where they stand and
// the ones a defer or go statement delays.
func (s *scanner) prepare(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.DeferStmt:
			if lit, ok := ast.Unparen(n.Call.Fun).(*ast.FuncLit); ok {
				s.skip[lit] = true
			}
		case *ast.GoStmt:
			if lit, ok := ast.Unparen(n.Call.Fun).(*ast.FuncLit); ok {
				s.skip[lit] = true
			}
		case *ast.CallExpr:
			if lit, ok := ast.Unparen(n.Fun).(*ast.FuncLit); ok {
				s.invoked[lit] = true
			}
		}
		return true
	})
}

// exits reports whether node has a call that runs the function's deferred
// calls.
func (s *scanner) exits(node ast.Node) bool {
	if node == nil {
		return false
	}
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return s.invoked[n] && !s.skip[n]
		case *ast.DeferStmt:
			if s.evaluated(n.Call) {
				found = true
			}
			return false
		case *ast.GoStmt:
			if s.evaluated(n.Call) {
				found = true
			}
			return false
		case *ast.CallExpr:
			if directUnwind(s.pass, n) {
				found = true
			}
		}
		return !found
	})
	return found
}

// evaluated reports whether the parts of call that run now have a call that
// runs the function's deferred calls. A deferred or started call itself does
// not run now, but its function value and arguments do.
func (s *scanner) evaluated(call *ast.CallExpr) bool {
	if _, ok := ast.Unparen(call.Fun).(*ast.FuncLit); !ok {
		if s.exits(call.Fun) {
			return true
		}
	}
	for _, arg := range call.Args {
		if s.exits(arg) {
			return true
		}
	}
	return false
}

// directUnwind reports whether call is a direct panic or runtime.Goexit call.
func directUnwind(pass *analysis.Pass, call *ast.CallExpr) bool {
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic") {
			return true
		}
	}
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "runtime" &&
		fn.Name() == "Goexit"
}

// walker walks the body of one loop, recording its defers and whether
// something in them can leave the loop or the function.
type walker struct {
	pass   *analysis.Pass
	loop   *ast.ForStmt
	labels map[string]*ast.LabeledStmt
	body   *ast.BlockStmt
	owners map[*ast.BranchStmt]ast.Node
	exits  bool
	defers []*ast.DeferStmt
}

// stmt walks one statement. depth counts the breakable statements between the
// statement and the loop body, so a bare break past a nested loop, switch, or
// select does not leave the outer loop.
func (w *walker) stmt(stmt ast.Stmt, depth int) {
	if stmt == nil || w.exits {
		return
	}
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		for _, child := range stmt.List {
			w.stmt(child, depth)
		}
	case *ast.LabeledStmt:
		w.stmt(stmt.Stmt, depth)
	case *ast.IfStmt:
		w.stmt(stmt.Body, depth)
		w.stmt(stmt.Else, depth)
	case *ast.ForStmt:
		w.stmt(stmt.Body, depth+1)
	case *ast.RangeStmt:
		w.stmt(stmt.Body, depth+1)
	case *ast.SwitchStmt:
		w.clauses(stmt.Body.List, depth+1)
	case *ast.TypeSwitchStmt:
		w.clauses(stmt.Body.List, depth+1)
	case *ast.SelectStmt:
		w.clauses(stmt.Body.List, depth+1)
	case *ast.CaseClause:
		for _, child := range stmt.Body {
			w.stmt(child, depth)
		}
	case *ast.CommClause:
		for _, child := range stmt.Body {
			w.stmt(child, depth)
		}
	case *ast.DeferStmt:
		w.defers = append(w.defers, stmt)
	case *ast.ReturnStmt:
		w.exits = true
	case *ast.BranchStmt:
		w.branch(stmt, depth)
	}
}

// clauses walks the clauses of a switch or a select.
func (w *walker) clauses(clauses []ast.Stmt, depth int) {
	for _, clause := range clauses {
		w.stmt(clause, depth)
	}
}

// branch records whether a break, continue, or goto leaves the loop.
func (w *walker) branch(stmt *ast.BranchStmt, depth int) {
	switch stmt.Tok {
	case token.BREAK:
		if stmt.Label == nil {
			if depth == 0 {
				w.exits = true
			}
			return
		}
		w.leaves(stmt.Label.Name, true)
	case token.CONTINUE:
		if stmt.Label != nil {
			w.leaves(stmt.Label.Name, false)
		}
	case token.GOTO:
		w.jumps(stmt.Label.Name)
	}
}

// leaves records whether a break or a continue to name leaves the loop. A
// break to a label on the loop leaves it, and so does either branch to a label
// outside the loop.
func (w *walker) leaves(name string, brk bool) {
	label, ok := w.labels[name]
	if !ok {
		w.exits = true
		return
	}
	if label.Stmt == w.loop {
		if brk {
			w.exits = true
		}
		return
	}
	if !w.inside(label.Pos()) {
		w.exits = true
	}
}

// jumps records whether a goto to name leaves the loop. A goto to a label on
// the loop or inside it starts another pass. A goto to a label before the loop
// does too, unless something between the label and the loop can leave.
func (w *walker) jumps(name string) {
	label, ok := w.labels[name]
	if !ok {
		w.exits = true
		return
	}
	if label.Stmt == w.loop || w.inside(label.Pos()) {
		return
	}
	before := label.Pos() < w.loop.Pos()
	stays := !w.escapes(label.Pos(), w.loop.Pos())
	if before && stays {
		return
	}
	w.exits = true
}

// leavesRegion reports whether branch leaves the source between from and to.
func (w *walker) leavesRegion(stmt *ast.BranchStmt, from token.Pos) bool {
	if stmt.Label != nil || stmt.Tok == token.GOTO {
		return true
	}
	owner := w.owners[stmt]
	return owner != nil && owner.Pos() <= from
}

// escapes reports whether the source between from and to has a return, a
// branch that leaves, a goto, or a direct panic or runtime.Goexit call. A
// function literal is skipped.
func (w *walker) escapes(from, to token.Pos) bool {
	var found bool
	ast.Inspect(w.body, func(n ast.Node) bool {
		if found || n == nil || n.Pos() >= to {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit, *ast.DeferStmt, *ast.GoStmt:
			return false
		case *ast.ReturnStmt:
			if n.Pos() >= from {
				found = true
			}
		case *ast.BranchStmt:
			if n.Pos() >= from && w.leavesRegion(n, from) {
				found = true
			}
		case *ast.CallExpr:
			if n.Pos() >= from && directUnwind(w.pass, n) {
				found = true
			}
		}
		return !found
	})
	return found
}

// inside reports whether pos is in the loop body.
func (w *walker) inside(pos token.Pos) bool {
	return w.loop.Body.Pos() <= pos && pos < w.loop.Body.End()
}
