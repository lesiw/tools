// Package loopvarcond provides an analysis.Analyzer that reports a for loop
// whose condition tests a variable the loop never changes.
//
// A for statement evaluates its condition before each iteration
// (https://go.dev/ref/spec#For_statements). A condition built from variables
// that the body and post statement leave alone evaluates to the same value at
// every test. The common fault is an increment of another variable:
//
//	for i := 0; i < n; j++ {
//		use(i)
//	}
//
// Taking the address of a tested variable before the loop ends, by &, a
// pointer-receiver method value (https://go.dev/ref/spec#Method_values), or a
// pointer-receiver method call (https://go.dev/ref/spec#Calls) leaves the loop
// alone, since a write through the pointer can change it without an assignment
// in the loop. A tested variable of struct or array type is left alone, since
// a field or element write changes it without an assignment to the variable.
// A variable that a function literal captures
// (https://go.dev/ref/spec#Function_literals) is left alone when a write can
// reach it while the loop runs.
//
// The rule is adapted from staticcheck's SA4008.
//
// The README catalogs every check and exemption.
//
// There is no fix: the tool cannot tell whether the author meant to change
// the tested variable or to leave the loop through the body.
package loopvarcond

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "loopvarcond",
	Doc:  "reports a for loop whose condition tests a variable the loop never changes",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		facts := inspect(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			loop, ok := n.(*ast.ForStmt)
			if ok {
				check(pass, loop, facts)
			}
			return true
		})
	}
	return nil, nil
}

// facts holds what a file does to a variable away from a single loop: takes
// its address, or captures it in a function literal.
type facts struct {
	addressed map[*types.Var][]token.Pos
	captured  map[*types.Var][]token.Pos
	written   map[*types.Var][]token.Pos
	literals  []*ast.FuncLit
}

// inspect collects the variables whose address or function-literal capture can
// change them while a loop runs.
func inspect(pass *analysis.Pass, file *ast.File) *facts {
	f := &facts{
		addressed: make(map[*types.Var][]token.Pos),
		captured:  make(map[*types.Var][]token.Pos),
		written:   make(map[*types.Var][]token.Pos),
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				if v := variable(pass, n.X); v != nil {
					f.addressed[v] = append(f.addressed[v], n.Pos())
				}
			}
		case *ast.FuncLit:
			f.literals = append(f.literals, n)
			capture(pass, n, f.captured)
		case *ast.SelectorExpr:
			address(pass, n, f.addressed)
		}
		return true
	})
	f.written = writePositions(pass, file)
	return f
}

// address records the variable a method value or call can change through a
// pointer.
func address(pass *analysis.Pass, sel *ast.SelectorExpr, out map[*types.Var][]token.Pos) {
	if !methodValue(pass, sel) {
		return
	}
	if v := variable(pass, sel.X); v != nil {
		out[v] = append(out[v], sel.Pos())
	}
}

// methodValue reports whether sel is a method value or call whose receiver is
// a pointer, which takes the address of the variable it is selected from.
func methodValue(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	s := pass.TypesInfo.Selections[sel]
	if s == nil || s.Kind() != types.MethodVal {
		return false
	}
	fn, ok := s.Obj().(*types.Func)
	if !ok {
		return false
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return false
	}
	if _, ok := recv.Type().Underlying().(*types.Pointer); !ok {
		return false
	}
	t := pass.TypesInfo.TypeOf(sel.X)
	if t == nil {
		return false
	}
	_, ok = t.Underlying().(*types.Pointer)
	return !ok
}

// capture records the variables that lit reads or writes from outside its own
// declaration. A value that a condition tests can be changed by another
// goroutine through the same variable.
func capture(pass *analysis.Pass, lit *ast.FuncLit, out map[*types.Var][]token.Pos) {
	ast.Inspect(lit, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		v, ok := pass.TypesInfo.ObjectOf(id).(*types.Var)
		if ok && (v.Pos() < lit.Pos() || v.Pos() > lit.End()) {
			out[v] = append(out[v], lit.Pos())
		}
		return true
	})
}

// before reports whether a position falls before end.
func before(positions []token.Pos, end token.Pos) bool {
	for _, pos := range positions {
		if pos < end {
			return true
		}
	}
	return false
}

// check reports loop when its condition is built from variables that the body
// and post statement never change.
func check(pass *analysis.Pass, loop *ast.ForStmt, f *facts) {
	if loop.Cond == nil || startsFalse(pass, loop) {
		return
	}
	vars, ok := stable(pass, loop.Cond)
	if !ok || len(vars) == 0 {
		return
	}
	for _, v := range vars {
		if global(v) || !whole(v.Type()) {
			return
		}
		if before(f.addressed[v], loop.End()) {
			return
		}
		if before(f.captured[v], loop.End()) && written(f, v, loop) {
			return
		}
	}
	changed := writes(pass, loop.Body)
	if postRuns(pass, loop) {
		for v := range writes(pass, loop.Post) {
			changed[v] = true
		}
	}
	for _, v := range vars {
		if changed[v] {
			return
		}
	}
	pass.Reportf(loop.Cond.Pos(), "loop condition never changes")
}

// written reports whether the file writes v while the loop can run.
func written(f *facts, v *types.Var, loop *ast.ForStmt) bool {
	var outer *ast.FuncLit
	for _, lit := range f.literals {
		if lit.Pos() <= loop.Pos() && loop.End() <= lit.End() {
			if outer == nil || lit.Pos() > outer.Pos() {
				outer = lit
			}
		}
	}
	for _, pos := range f.written[v] {
		if outer != nil && (pos < outer.Pos() || pos > outer.End()) {
			return true // the surrounding function can write beside the loop
		}
		if pos < loop.End() {
			return true
		}
	}
	return false
}

// env holds the constant values an init statement settles.
type env map[*types.Var]constant.Value

// startsFalse reports whether a constant init statement gives the tested
// variable a value that makes the condition false at the first test.
func startsFalse(pass *analysis.Pass, loop *ast.ForStmt) bool {
	init, ok := loop.Init.(*ast.AssignStmt)
	if !ok || len(init.Lhs) != len(init.Rhs) {
		return false
	}
	switch init.Tok {
	case token.DEFINE, token.ASSIGN:
	default:
		return false // a compound assignment does not hold the RHS value
	}
	known := make(env)
	for i, lhs := range init.Lhs {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok {
			continue
		}
		value := pass.TypesInfo.Types[init.Rhs[i]].Value
		v, ok := pass.TypesInfo.ObjectOf(id).(*types.Var)
		if !ok {
			continue
		}
		if value != nil {
			known[v] = value
		} else {
			delete(known, v)
		}
	}
	if len(known) == 0 {
		return false
	}
	value, ok := eval(pass, loop.Cond, known)
	return ok && value.Kind() == constant.Bool && !constant.BoolVal(value)
}

// eval returns the constant value of e, using known for the variables the init
// statement settles. It reports false for a value it cannot settle.
func eval(pass *analysis.Pass, e ast.Expr, known env) (constant.Value, bool) {
	if tv, ok := pass.TypesInfo.Types[e]; ok && tv.Value != nil {
		return tv.Value, true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := pass.TypesInfo.ObjectOf(e).(*types.Var)
		if !ok {
			return nil, false
		}
		value, ok := known[v]
		return value, ok
	case *ast.UnaryExpr:
		x, ok := eval(pass, e.X, known)
		if !ok {
			return nil, false
		}
		if e.Op == token.NOT {
			return constant.UnaryOp(e.Op, x, 0), true
		}
	case *ast.BinaryExpr:
		return binary(pass, e, known)
	}
	return nil, false
}

// binary returns the constant value of a binary expression.
func binary(pass *analysis.Pass, e *ast.BinaryExpr, known env) (constant.Value, bool) {
	if e.Op == token.LAND || e.Op == token.LOR {
		return logical(pass, e, known)
	}
	var (
		x, xok = eval(pass, e.X, known)
		y, yok = eval(pass, e.Y, known)
	)
	if !xok || !yok || x.Kind() != y.Kind() {
		return nil, false
	}
	switch e.Op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return constant.MakeBool(constant.Compare(x, e.Op, y)), true
	}
	return nil, false
}

// logical returns the constant value of a && or || expression.
func logical(pass *analysis.Pass, e *ast.BinaryExpr, known env) (constant.Value, bool) {
	x, xok := eval(pass, e.X, known)
	if settles(e.Op, x, xok) {
		return constant.MakeBool(e.Op == token.LOR), true
	}
	if !boolean(x, xok) {
		return nil, false // the left operand can panic or decide the result
	}
	y, yok := eval(pass, e.Y, known)
	if settles(e.Op, y, yok) {
		return constant.MakeBool(e.Op == token.LOR), true
	}
	if boolean(x, xok) && boolean(y, yok) {
		return constant.MakeBool(e.Op == token.LAND), true
	}
	return nil, false
}

// settles reports whether x fixes the result of op: false for &&, true for
// ||.
func settles(op token.Token, x constant.Value, ok bool) bool {
	if !boolean(x, ok) {
		return false
	}
	if op == token.LAND {
		return !constant.BoolVal(x)
	}
	if op == token.LOR {
		return constant.BoolVal(x)
	}
	return false
}

// boolean reports whether x is a known boolean value.
func boolean(x constant.Value, ok bool) bool {
	return ok && x.Kind() == constant.Bool
}

// stable returns the variables that e tests. It reports false unless e is a
// variable, a constant, a literal, a conversion of a whole value, or an
// operator expression over them.
func stable(pass *analysis.Pass, e ast.Expr) ([]*types.Var, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		switch obj := pass.TypesInfo.ObjectOf(e).(type) {
		case *types.Var:
			return []*types.Var{obj}, true
		case *types.Const, *types.Nil:
			return nil, true
		}
	case *ast.BasicLit:
		return nil, true
	case *ast.UnaryExpr:
		switch e.Op {
		case token.NOT, token.SUB, token.ADD, token.XOR:
			return stable(pass, e.X)
		}
	case *ast.BinaryExpr:
		x, ok := stable(pass, e.X)
		if !ok {
			return nil, false
		}
		y, ok := stable(pass, e.Y)
		if !ok {
			return nil, false
		}
		return append(x, y...), true
	case *ast.SelectorExpr:
		if _, ok := pass.TypesInfo.Uses[e.Sel].(*types.Const); ok {
			return nil, true
		}
	case *ast.CallExpr:
		if x, ok := conversion(pass, e); ok {
			return stable(pass, x)
		}
	}
	return nil, false
}

// conversion returns the operand when call converts a whole value.
func conversion(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() || len(call.Args) != 1 {
		return nil, false
	}
	if !plain(pass.TypesInfo.TypeOf(call.Args[0])) {
		return nil, false
	}
	return call.Args[0], true
}

// plain reports whether a value of type t is its whole value, so that a
// conversion of it cannot copy the contents of a slice, array, or struct.
func plain(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Basic, *types.Pointer, *types.Chan, *types.Signature:
		return true
	}
	return false
}

// global reports whether v belongs to a package, where another goroutine can
// write it without a write in the loop.
func global(v *types.Var) bool {
	return v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

// whole reports whether a value of type t can change only by writing the
// variable itself. A struct or array value also changes when a field or
// element is written, and a type parameter can hold a type the rule does not
// read.
func whole(t types.Type) bool {
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return false
	}
	switch t.Underlying().(type) {
	case *types.Struct, *types.Array:
		return false
	}
	return true
}

// continues reports whether body holds a continue that can reach the post
// statement. An unlabelled continue in a nested loop belongs to that loop, and
// a labeled one may name this loop.
func continues(body *ast.BlockStmt) bool {
	var (
		found bool
		walk  func(ast.Node, bool)
	)
	walk = func(n ast.Node, nested bool) {
		ast.Inspect(n, func(n ast.Node) bool {
			if found {
				return false
			}
			switch n := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ForStmt:
				walk(n.Body, true)
				return false
			case *ast.RangeStmt:
				walk(n.Body, true)
				return false
			case *ast.BranchStmt:
				found = n.Tok == token.CONTINUE && (!nested || n.Label != nil)
			}
			return true
		})
	}
	walk(body, false)
	return found
}

// postRuns reports whether the post statement can run.
func postRuns(pass *analysis.Pass, loop *ast.ForStmt) bool {
	return !exits(pass, loop.Body.List) || continues(loop.Body) ||
		gotos(loop.Body)
}

// gotos reports whether body holds a goto. The label makes reachability hard
// to read, so the post statement is not skipped for a body with one.
func gotos(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			found = n.Tok == token.GOTO
		}
		return true
	})
	return found
}

// exits reports whether the statement list always leaves before its end.
func exits(pass *analysis.Pass, stmts []ast.Stmt) bool {
	for _, stmt := range stmts {
		if leaves(pass, stmt) {
			return true
		}
	}
	return false
}

// leaves reports whether stmt always leaves the statement list.
func leaves(pass *analysis.Pass, stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok == token.BREAK
	case *ast.BlockStmt:
		return exits(pass, stmt.List)
	case *ast.IfStmt:
		return stmt.Else != nil && exits(pass, stmt.Body.List) &&
			leaves(pass, stmt.Else)
	case *ast.ExprStmt:
		return panics(pass, stmt.X)
	}
	return false
}

// panics reports whether e calls the builtin panic.
func panics(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	return pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic")
}

// variable returns the variable that e names, if it names one.
func variable(pass *analysis.Pass, e ast.Expr) *types.Var {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	v, _ := pass.TypesInfo.ObjectOf(id).(*types.Var)
	return v
}

// writePositions returns the positions where node writes a variable.
func writePositions(pass *analysis.Pass, node ast.Node) map[*types.Var][]token.Pos {
	out := make(map[*types.Var][]token.Pos)
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				break
			}
			for _, lhs := range n.Lhs {
				if v := variable(pass, lhs); v != nil {
					out[v] = append(out[v], n.Pos())
				}
			}
		case *ast.IncDecStmt:
			if v := variable(pass, n.X); v != nil {
				out[v] = append(out[v], n.Pos())
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				if v := variable(pass, n.Key); v != nil {
					out[v] = append(out[v], n.Pos())
				}
				if v := variable(pass, n.Value); v != nil {
					out[v] = append(out[v], n.Pos())
				}
			}
		}
		return true
	})
	return out
}

// writes returns the variables that node changes: by assignment, increment,
// decrement, or range assignment.
func writes(pass *analysis.Pass, node ast.Node) map[*types.Var]bool {
	out := make(map[*types.Var]bool)
	if node == nil {
		return out
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				break // a declaration is not a change to the outer variable
			}
			for _, lhs := range n.Lhs {
				if v := variable(pass, lhs); v != nil {
					out[v] = true
				}
			}
		case *ast.IncDecStmt:
			if v := variable(pass, n.X); v != nil {
				out[v] = true
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				if v := variable(pass, n.Key); v != nil {
					out[v] = true
				}
				if v := variable(pass, n.Value); v != nil {
					out[v] = true
				}
			}
		}
		return true
	})
	return out
}
