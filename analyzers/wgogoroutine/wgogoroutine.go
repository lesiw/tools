// Package wgogoroutine provides an analysis.Analyzer that reports a
// sync.WaitGroup's count raised from inside a goroutine instead of before it
// starts.
//
// Add's documentation says "calls with a positive delta that occur when the
// counter is zero must happen before a Wait", and that "typically this means
// the calls to Add should execute before the statement creating the goroutine
// or other event to be waited for". The literal shows no order between an Add
// it makes and a Wait that follows, so a Wait can observe a zero counter
// before the call raises it. It then returns without the work, and an Add that
// raises the count from zero while a Wait is blocked panics.
// WaitGroup.Go raises the same count, and its documentation says "If the
// WaitGroup is empty, Go must happen before a Wait".
//
// Report only: hoisting the count above the statement that starts the
// goroutine changes what the group counts whenever the call is conditional or
// sits in a loop. Where the group is a parameter of the literal, the hoist
// would have to write the argument's name at the call site, which the tool
// cannot invent. WaitGroup.Go, added in Go 1.25, raises the count before it
// starts the task and needs no hoist, but it counts the task it starts, and
// the Add may be counting work that the task begins.
//
// The README catalogs every check and exemption.
//
// The rule is adapted from the x/tools vet pass waitgroup and from
// staticcheck's SA2000, both of which read an Add written as the first
// statement of the goroutine's literal. This rule reads the whole body of the
// literal, and stops where a nested literal begins.
package wgogoroutine

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "wgogoroutine",
	Doc:  "reports a count raised inside a goroutine instead of before it starts",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			if lit, ok := ast.Unparen(stmt.Call.Fun).(*ast.FuncLit); ok {
				report(pass, lit)
			}
			return true
		})
	}
	return nil, nil
}

// call is a call lit makes on a WaitGroup, and where the literal writes it.
type call struct {
	recv     ast.Expr
	name     string
	delta    ast.Expr
	pos      token.Pos
	end      token.Pos
	deferred bool
}

// raises reports whether the call raises a WaitGroup's count. A call to Add
// with a constant delta of zero or less does not, and a delta that is not
// constant may.
func raises(pass *analysis.Pass, c call) bool {
	switch c.name {
	case "Add":
		d, known := sign(pass, c.delta)
		return !known || d > 0
	case "Go":
		return true
	}
	return false
}

// report reports a count lit raises unless the literal creates the group,
// lowers the group's count with a deferred call first, or waits on it after.
func report(pass *analysis.Pass, lit *ast.FuncLit) {
	calls := callsOf(pass, lit)
	for _, c := range calls {
		if !raises(pass, c) {
			continue
		}
		if created(pass, c.recv, lit) {
			continue
		}
		if lowered(pass, c, calls) || waitedAfter(pass, c, calls) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos: c.pos,
			End: c.end,
			Message: c.name + " raised from inside " +
				"a goroutine instead of before it starts",
		})
	}
}

// has reports whether set holds expr.
func has(set map[*ast.CallExpr]struct{}, expr *ast.CallExpr) bool {
	_, ok := set[expr]
	return ok
}

// callsOf returns the calls lit makes on a WaitGroup, in source order, without
// the ones a nested literal makes.
func callsOf(pass *analysis.Pass, lit *ast.FuncLit) (out []call) {
	for _, stmt := range lit.Body.List {
		deferred := make(map[*ast.CallExpr]struct{})
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.DeferStmt:
				deferred[n.Call] = struct{}{}
			}
			expr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			recv, name, delta, ok := waitGroupCall(pass, expr)
			if !ok {
				return true
			}
			out = append(out, call{
				recv:     recv,
				name:     name,
				delta:    delta,
				pos:      expr.Pos(),
				end:      expr.End(),
				deferred: has(deferred, expr),
			})
			return true
		})
	}
	return out
}

// lowered reports whether the last call lit makes that lowers the count of the
// group c names, before c, is deferred. A goroutine that defers a lower before
// it raises the count holds a count of its own until it returns, and Add's
// documentation permits a positive delta while the counter is above zero.
func lowered(pass *analysis.Pass, c call, calls []call) bool {
	last := -1
	for i, other := range calls {
		if other.pos >= c.pos || !lowers(pass, other) {
			continue
		}
		if same(pass, other.recv, c.recv) {
			last = i
		}
	}
	return last >= 0 && calls[last].deferred
}

// lowers reports whether the call lowers a WaitGroup's count.
func lowers(pass *analysis.Pass, c call) bool {
	switch c.name {
	case "Done":
		return true
	case "Add":
		d, known := sign(pass, c.delta)
		return known && d < 0
	}
	return false
}

// waitedAfter reports whether lit waits on c's group after c, which covers the
// count c raises. A deferred wait runs at return, so it covers a call written
// before it too.
func waitedAfter(pass *analysis.Pass, c call, calls []call) bool {
	return slices.ContainsFunc(calls, func(other call) bool {
		if other.name != "Wait" || (!other.deferred && other.pos <= c.pos) {
			return false
		}
		return same(pass, other.recv, c.recv)
	})
}

// same reports whether a and b name the same group, however the receiver is
// written.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	return astexpr.Same(pass, bare(a), bare(b))
}

// bare returns e without the parentheses, dereferences, and addresses wrapped
// around it, which name the same value.
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

// created reports whether the group recv names is created in lit's body, where
// nothing can have waited on it before the literal made it.
func created(pass *analysis.Pass, recv ast.Expr, lit *ast.FuncLit) bool {
	for {
		switch e := bare(recv).(type) {
		case *ast.CompositeLit:
			return true
		case *ast.CallExpr:
			return creates(pass, e)
		case *ast.Ident:
			return declaredNew(pass, pass.TypesInfo.ObjectOf(e), lit)
		case *ast.SelectorExpr:
			recv = e.X
		default:
			return false
		}
	}
}

// declaredNew reports whether lit's body declares obj, and gives it a value
// the literal creates rather than a group from outside.
func declaredNew(pass *analysis.Pass, obj types.Object, lit *ast.FuncLit) bool {
	var made bool
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if pass.TypesInfo.Defs[name] != obj {
					continue
				}
				made = len(n.Values) == 0 ||
					(len(n.Values) == len(n.Names) &&
						creates(pass, n.Values[i]))
				return false
			}
		case *ast.AssignStmt:
			// A call that returns several values gives each name a value
			// this rule cannot see into.
			if len(n.Rhs) != len(n.Lhs) {
				return false
			}
			for i, lhs := range n.Lhs {
				ident, ok := ast.Unparen(lhs).(*ast.Ident)
				if !ok || pass.TypesInfo.Defs[ident] != obj {
					continue
				}
				made = creates(pass, n.Rhs[i])
				return false
			}
		}
		return true
	})
	return made
}

// creates reports whether the value creates a WaitGroup where it is written.
func creates(pass *analysis.Pass, value ast.Expr) bool {
	v := ast.Unparen(value)
	if unary, ok := v.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		v = ast.Unparen(unary.X)
	}
	switch v := v.(type) {
	case *ast.CompositeLit:
		return true
	case *ast.CallExpr:
		ident, ok := ast.Unparen(v.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		_, ok = pass.TypesInfo.Uses[ident].(*types.Builtin)
		return ok && ident.Name == "new"
	}
	return false
}

// sign returns the direction of the delta a call to Add passes: -1, 0, or 1.
// It reports false for a delta that is not constant, which may be any of them.
func sign(pass *analysis.Pass, delta ast.Expr) (int, bool) {
	if delta == nil {
		return 0, false
	}
	tv, ok := pass.TypesInfo.Types[delta]
	if !ok || tv.Value == nil {
		return 0, false
	}
	return constant.Sign(tv.Value), true
}

// waitGroupCall returns the receiver of a call to a method of sync.WaitGroup,
// the name of the method, and the first argument after the receiver. A method
// expression carries its receiver as the first argument.
func waitGroupCall(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, string, ast.Expr, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, "", nil, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil, "", nil, false
	}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil || !waitGroupPointer(sig.Recv().Type()) {
		return nil, "", nil, false
	}
	recv, args := sel.X, call.Args
	if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.IsType() {
		if len(args) == 0 {
			return nil, "", nil, false
		}
		recv, args = args[0], args[1:]
	}
	var delta ast.Expr
	if len(args) > 0 {
		delta = args[0]
	}
	return recv, fn.Name(), delta, true
}

// waitGroupPointer reports whether typ is *sync.WaitGroup. Every method of the
// type has that receiver, including the ones a value or an embedded field
// reaches, so one test covers them all.
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
