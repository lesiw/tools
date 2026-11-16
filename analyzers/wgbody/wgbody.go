// Package wgbody provides an analysis.Analyzer that reports a Done call or a
// panic inside a sync.WaitGroup.Go body.
//
// WaitGroup.Go's documentation (https://pkg.go.dev/sync#WaitGroup.Go) says
// "Go calls f in a new goroutine and adds that task to the WaitGroup. When f
// returns, the task is removed from the WaitGroup." A Done call inside f
// removes the task Go added. If it runs before f returns, a Wait can return
// while f still runs. WaitGroup.Go removes the task again when f returns, and
// the count can become negative. The same documentation says "The function f
// must not panic."
//
// WaitGroup.Go was added in Go 1.25.
//
// A Done on the same WaitGroup is reported unless an earlier, non-deferred,
// non-goroutine Add on that group is in the same block or clause, or an
// enclosing block. The Add must be in the declared function that contains the
// Go call, or in the outermost function literal when there is no declaration.
// A constant Add delta covers that many Done calls, and a non-constant delta
// covers any number; a Done written once counts once even if a loop runs it
// more than once. A Done outside the Go body and a negative Add do not reduce
// an Add's capacity. An Add in a branch that does not enclose the Done does
// not cover it. An Add on another WaitGroup does not cover it, and neither
// does an Add with a constant delta of zero or less. A Done on another
// WaitGroup is not reported. A Done or Add is recognized only when its
// receiver is the same group value, allowing parentheses, a dereference, or an
// address; a
// second name for the group is not recognized. A method expression, an
// embedded WaitGroup, and a type alias are recognized.
//
// A Done and a panic are read in the statements of the body, in a direct call
// made by a go statement, and in a function literal that is the callee of a
// call or defer there. A literal passed to another function, including another
// Go call, assigned to a variable, or started with go is not followed. The
// builtin panic and the log.Panic variants are read. A panic after a deferred
// function literal that calls recover in an expression statement, an
// assignment, or an if, switch, or type-switch init before the panic is not
// reported; a recover under a branch, after the panic, or inside a nested
// literal does not count. A deliberate panic can be marked with the suite's
// //ignore:wgbody directive.
//
// Only a function literal passed to Go is read. A function name, a method
// value, or any other value passed to Go is not read.
//
// Report only: deleting a Done can leave a count that an Add in another
// function raised, and deleting a panic changes what the body does when it
// panics.
//
// The rule is inspired by revive's forbidden-call-in-wg-go.
package wgbody

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "wgbody",
	Doc:  "reports a Done call or a panic inside a sync.WaitGroup.Go body",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		used := make(map[*ast.CallExpr]int)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			wg, task, ok := goCall(pass, call)
			if !ok {
				return true
			}
			lit, ok := ast.Unparen(task).(*ast.FuncLit)
			if !ok {
				return true
			}
			check(pass, file, wg, lit, used)
			return true
		})
	}
	return nil, nil
}

// check reports the Done calls and panics found in lit.
func check(pass *analysis.Pass, file *ast.File, wg ast.Expr, lit *ast.FuncLit, used map[*ast.CallExpr]int) {
	var (
		doned, panicked = callsOf(pass, file, lit, wg)
		exempt          = exemptions(
			pass, wg, waitGroupCalls(pass, enclosingFunc(file, lit)),
			doned, file, used,
		)
	)
	for _, call := range doned {
		if exempt[call] {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "Done inside a WaitGroup.Go body removes the task Go added",
		})
	}
	for _, call := range panicked {
		pass.Report(analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "panic inside a WaitGroup.Go body, which must not panic",
		})
	}
}

// enclosingFunc returns the declared function that contains lit, or the
// outermost function literal when there is no declaration.
func enclosingFunc(file *ast.File, lit *ast.FuncLit) (outer ast.Node) {
	path, _ := astutil.PathEnclosingInterval(file, lit.Pos(), lit.End())
	for _, n := range path {
		switch n := n.(type) {
		case *ast.FuncDecl:
			return n
		case *ast.FuncLit:
			if n != lit {
				outer = n
			}
		}
	}
	if outer == nil {
		return file
	}
	return outer
}

// call is a WaitGroup method call in the enclosing function.
type call struct {
	expr     *ast.CallExpr
	recv     ast.Expr
	method   string
	delta    ast.Expr
	deferred bool
	async    bool
}

// waitGroupCalls returns the WaitGroup calls in root.
func waitGroupCalls(pass *analysis.Pass, root ast.Node) (out []call) {
	var (
		deferred = make(map[*ast.CallExpr]struct{})
		async    = make(map[*ast.CallExpr]struct{})
	)
	ast.Inspect(root, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.DeferStmt:
			deferred[n.Call] = struct{}{}
		case *ast.GoStmt:
			async[n.Call] = struct{}{}
		case *ast.CallExpr:
			recv, name, args, ok := waitGroupCall(pass, n)
			if !ok {
				return true
			}
			var delta ast.Expr
			if len(args) > 0 {
				delta = args[0]
			}
			_, isDeferred := deferred[n]
			_, isAsync := async[n]
			out = append(out, call{
				expr:     n,
				recv:     recv,
				method:   name,
				delta:    delta,
				deferred: isDeferred,
				async:    isAsync,
			})
		}
		return true
	})
	return out
}

// exemptions returns the Done calls that an earlier Add may cover. An Add
// covers a Done when it is in the same block as the Done or one that encloses
// it, it runs before the Done, and it is not deferred. A constant delta covers
// that many Done calls, and a non-constant delta covers any number.
func exemptions(pass *analysis.Pass, wg ast.Expr, calls []call, doned []*ast.CallExpr, file *ast.File, used map[*ast.CallExpr]int) map[*ast.CallExpr]bool {
	out := make(map[*ast.CallExpr]bool)
	for _, done := range doned {
		for _, add := range slices.Backward(calls) {
			if !covers(pass, add, done, wg, file) || !take(pass, add, used) {
				continue
			}
			out[done] = true
			break
		}
	}
	return out
}

// covers reports whether add may cover the Done that done lowers.
func covers(pass *analysis.Pass, add call, done *ast.CallExpr, wg ast.Expr, file *ast.File) bool {
	if add.method != "Add" || add.deferred || add.async {
		return false
	}
	if add.expr.Pos() >= done.Pos() {
		return false
	}
	if !same(pass, add.recv, wg) || !positive(pass, add.delta) {
		return false
	}
	block := enclosing(add.expr, file)
	return block != nil && inPath(done, file, block)
}

// take uses one of the counts add raised. It reports false when add covers no
// more Done calls.
func take(pass *analysis.Pass, add call, used map[*ast.CallExpr]int) bool {
	n, limited := capacity(pass, add.delta)
	if !limited {
		return true
	}
	if used[add.expr] >= n {
		return false
	}
	used[add.expr]++
	return true
}

// capacity returns the number of Done calls an Add covers. A delta that is not
// a constant int covers any number.
func capacity(pass *analysis.Pass, delta ast.Expr) (int, bool) {
	tv, ok := pass.TypesInfo.Types[delta]
	if !ok || tv.Value == nil {
		return 0, false
	}
	n, ok := constant.Int64Val(tv.Value)
	if !ok || int64(int(n)) != n {
		return 0, false
	}
	return int(n), true
}

// enclosing returns the innermost block or clause that contains call.
func enclosing(call *ast.CallExpr, file *ast.File) ast.Node {
	path, _ := astutil.PathEnclosingInterval(file, call.Pos(), call.End())
	for _, n := range path {
		switch n.(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
			return n
		}
	}
	return nil
}

// inPath reports whether scope is one of the scopes that enclose call.
func inPath(call *ast.CallExpr, file *ast.File, scope ast.Node) bool {
	path, _ := astutil.PathEnclosingInterval(file, call.Pos(), call.End())
	return slices.Contains(path, scope)
}

// callsOf returns the Done calls on wg and the panic calls in lit, in a direct
// call made by a go statement, and in a function literal that is the callee of
// a call or defer there. A literal passed elsewhere, assigned to a variable,
// or started with go is not followed.
func callsOf(pass *analysis.Pass, file *ast.File, lit *ast.FuncLit, wg ast.Expr) (doned, panicked []*ast.CallExpr) {
	var walk func(*ast.BlockStmt, bool)
	walk = func(body *ast.BlockStmt, safe bool) {
		ast.Inspect(body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.GoStmt:
				if _, ok := ast.Unparen(n.Call.Fun).(*ast.FuncLit); ok {
					return false
				}
				done, pan := direct(pass, n.Call, wg)
				if done != nil {
					doned = append(doned, done)
				}
				if pan != nil {
					panicked = append(panicked, pan)
				}
				return false
			case *ast.FuncLit:
				if invoked(file, n) {
					walk(n.Body, safe || recovered(pass, body, n.Pos()))
				}
				return false
			case *ast.CallExpr:
				done, pan := direct(pass, n, wg)
				if done != nil {
					doned = append(doned, done)
				}
				covered := safe || recovered(pass, body, n.Pos())
				if pan != nil && !covered {
					panicked = append(panicked, pan)
				}
			}
			return true
		})
	}
	walk(lit.Body, false)
	return doned, panicked
}

// direct returns the Done or panic call that call makes on wg.
func direct(pass *analysis.Pass, call *ast.CallExpr, wg ast.Expr) (done, pan *ast.CallExpr) {
	if panics(pass, call) {
		return nil, call
	}
	recv, name, args, ok := waitGroupCall(pass, call)
	if !ok || name != "Done" || len(args) != 0 {
		return nil, nil
	}
	if same(pass, recv, wg) {
		return call, nil
	}
	return nil, nil
}

// recovered reports whether body defers a function literal that calls recover
// before pos. A defer that contains pos does not count.
func recovered(pass *analysis.Pass, body *ast.BlockStmt, pos token.Pos) bool {
	for _, stmt := range body.List {
		deferStmt, ok := stmt.(*ast.DeferStmt)
		if !ok || deferStmt.Pos() > pos || pos < deferStmt.End() {
			continue
		}
		inner, ok := ast.Unparen(deferStmt.Call.Fun).(*ast.FuncLit)
		if ok && directRecover(pass, inner.Body) {
			return true
		}
	}
	return false
}

// directRecover reports whether body calls recover as a statement of its own.
// A recover under a branch does not count, since the branch may not run.
func directRecover(pass *analysis.Pass, body *ast.BlockStmt) bool {
	return slices.ContainsFunc(body.List, func(stmt ast.Stmt) bool {
		switch stmt := stmt.(type) {
		case *ast.ExprStmt:
			call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
			return ok && isRecover(pass, call)
		case *ast.AssignStmt:
			return rhsRecovers(pass, stmt.Rhs)
		case *ast.IfStmt:
			init, ok := stmt.Init.(*ast.AssignStmt)
			return ok && rhsRecovers(pass, init.Rhs)
		case *ast.SwitchStmt:
			init, ok := stmt.Init.(*ast.AssignStmt)
			return ok && rhsRecovers(pass, init.Rhs)
		case *ast.TypeSwitchStmt:
			init, ok := stmt.Init.(*ast.AssignStmt)
			return ok && rhsRecovers(pass, init.Rhs)
		}
		return false
	})
}

// rhsRecovers reports whether one of the expressions on the right of an
// assignment is a call to recover.
func rhsRecovers(pass *analysis.Pass, rhs []ast.Expr) bool {
	return slices.ContainsFunc(rhs, func(expr ast.Expr) bool {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		return ok && isRecover(pass, call)
	})
}

// isRecover reports whether call is the builtin recover.
func isRecover(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || ident.Name != "recover" {
		return false
	}
	_, ok = pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok
}

// invoked reports whether lit is the callee of a call, directly or under
// parentheses.
func invoked(file *ast.File, lit *ast.FuncLit) bool {
	path, _ := astutil.PathEnclosingInterval(file, lit.Pos(), lit.End())
	for i, n := range path {
		if n != lit {
			continue
		}
		for j := i + 1; j < len(path); j++ {
			switch x := path[j].(type) {
			case *ast.ParenExpr:
			case *ast.CallExpr:
				return ast.Unparen(x.Fun) == lit
			default:
				return false
			}
		}
	}
	return false
}

// panics reports whether call is the builtin panic or a log.Panic variant.
func panics(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if ok && ident.Name == "panic" {
		_, ok = pass.TypesInfo.Uses[ident].(*types.Builtin)
		return ok
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "log" {
		return false
	}
	switch fn.Name() {
	case "Panic", "Panicf", "Panicln":
		return true
	}
	return false
}

// positive reports whether an Add's delta raises the count. A delta that is
// not constant may raise it.
func positive(pass *analysis.Pass, delta ast.Expr) bool {
	if delta == nil {
		return false
	}
	tv, ok := pass.TypesInfo.Types[delta]
	if !ok || tv.Value == nil {
		return true
	}
	return constant.Sign(tv.Value) > 0
}

// goCall returns the WaitGroup a call starts a task on and the task it takes.
func goCall(pass *analysis.Pass, call *ast.CallExpr) (wg, task ast.Expr, ok bool) {
	recv, name, args, ok := waitGroupCall(pass, call)
	if !ok || name != "Go" {
		return nil, nil, false
	}
	return recv, args[0], true
}

// waitGroupCall returns the receiver and the name of a call to Add, Done, or
// Go on a sync.WaitGroup, and the arguments after the receiver. In a method
// expression the receiver is the first argument.
func waitGroupCall(pass *analysis.Pass, call *ast.CallExpr) (recv ast.Expr, name string, args []ast.Expr, ok bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, "", nil, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "sync" {
		return nil, "", nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || !waitGroupPointer(sig.Recv().Type()) {
		return nil, "", nil, false
	}
	switch fn.Name() {
	case "Add", "Go":
	case "Done":
	default:
		return nil, "", nil, false
	}
	recv, args = sel.X, call.Args
	selection := pass.TypesInfo.Selections[sel]
	if selection != nil && selection.Kind() == types.MethodExpr {
		if len(args) == 0 {
			return nil, "", nil, false
		}
		recv, args = args[0], args[1:]
	}
	switch fn.Name() {
	case "Add", "Go":
		if len(args) != 1 {
			return nil, "", nil, false
		}
	case "Done":
		if len(args) != 0 {
			return nil, "", nil, false
		}
	}
	return recv, fn.Name(), args, true
}

// same reports whether a and b are the same group expression. A copy of the
// receiver is a different expression.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	return astexpr.Same(pass, bare(a), bare(b))
}

// bare returns e without the parentheses, dereferences, and addresses wrapped
// around it.
func bare(e ast.Expr) ast.Expr {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return x
			}
			e = x.X
		default:
			return x
		}
	}
}

// waitGroupPointer reports whether typ is *sync.WaitGroup.
func waitGroupPointer(typ types.Type) bool {
	ptr, ok := types.Unalias(typ).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "WaitGroup" && obj.Pkg() != nil &&
		obj.Pkg().Path() == "sync"
}
