// Package badsyncfunc provides an analysis.Analyzer that reports a
// sync.Once.Do call whose function reads a variable that a loop enclosing the
// call declares.
//
// sync.Once's doc says that "a new instance of Once is required for each
// function to execute", and that "if once.Do(f) is called multiple times, only
// the first call will invoke f, even if f has a different value in each
// invocation." A loop that passes a new function on every iteration and
// reuses the Once therefore invokes the first iteration's function and no
// other.
//
// The rule reports a Do call in that position when the Once it is on is one
// the loop reuses; a Once the loop builds each iteration is not reported. A
// variable that a function literal between the loop and the call declares is
// skipped.
//
// Report only: the repair is either to move the Do call out of the loop or to
// create the Once inside it, and either edit changes what the program does.
//
// The rule is inspired by go-critic's badSyncOnceFunc, which reports a
// sync.OnceFunc value that is discarded or called where it is built.
//
// The README catalogs every check and exemption.
package badsyncfunc

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "badsyncfunc",
	Doc:  "reports a sync.Once.Do call that keeps one iteration's value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports call when it names sync.Once's Do, a loop declares a value its
// function reads, and the loop neither declares the Once, writes it, nor
// passes a pointer to it.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.FullName() != "(*sync.Once).Do" {
		return
	}
	// Do takes one function, so a second argument is the receiver that a
	// method expression passes first.
	recv, do := sel.X, call.Args[0]
	if len(call.Args) == 2 {
		recv, do = call.Args[0], call.Args[1]
	}
	lit, ok := ast.Unparen(do).(*ast.FuncLit)
	if !ok {
		return
	}
	path, _ := astutil.PathEnclosingInterval(file, call.Pos(), call.End())
	s := scopeOf(pass, path, sel)
	loop, value := s.loopOf(lit)
	if loop == nil {
		return
	}
	once := root(pass, recv)
	if once == nil {
		return
	}
	// A loop that declares the Once, writes it, or passes a pointer to it
	// leaves every iteration with a fresh one.
	if s.declares(once, loop) || s.rebuilds(recv, loop, call) {
		return
	}
	pass.Reportf(call.Pos(),
		"sync.Once.Do runs the function once, so one iteration's %s is used",
		value.Name,
	)
}

// scope holds the nesting a call sits in: the loops that enclose it, innermost
// first, the function literals around it, and the fields a promoted Do is
// reached through, if any.
type scope struct {
	pass   *analysis.Pass
	loops  []ast.Node
	funcs  []ast.Node
	fields []*types.Var
}

// scopeOf returns the loops and function literals of path.
func scopeOf(pass *analysis.Pass, path []ast.Node, sel *ast.SelectorExpr) scope {
	s := scope{pass: pass, fields: promoted(pass, sel)}
	for _, n := range path {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			s.loops = append(s.loops, n)
		case *ast.FuncLit:
			s.funcs = append(s.funcs, n)
		}
	}
	return s
}

// encloses reports whether pos falls within n's source range.
func encloses(n ast.Node, pos token.Pos) bool {
	return pos.IsValid() && n.Pos() <= pos && pos < n.End()
}

// loopOf returns the innermost loop that supplies a value lit reads, and the
// identifier that reads it, reporting a nil loop when no loop supplies one.
func (s scope) loopOf(lit *ast.FuncLit) (ast.Node, *ast.Ident) {
	for _, loop := range s.loops {
		if id := s.supplies(loop, lit); id != nil {
			return loop, id
		}
	}
	return nil, nil
}

// supplies returns the identifier that reads a value loop declares, and nil
// when the literal reads no variable the loop declares.
func (s scope) supplies(loop ast.Node, lit *ast.FuncLit) *ast.Ident {
	var (
		written = targets(lit)
		found   *ast.Ident
	)
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok || s.writesTarget(written, id) {
			return true
		}
		v, ok := s.pass.TypesInfo.Uses[id].(*types.Var)
		if !ok || v.IsField() || encloses(lit, v.Pos()) {
			return true
		}
		if !encloses(loop, v.Pos()) {
			return true
		}
		for _, fn := range s.funcs {
			if encloses(loop, fn.Pos()) && encloses(fn, v.Pos()) {
				return true
			}
		}
		found = id
		return false
	})
	return found
}

// promoted returns the fields a promoted Do is reached through, in order, and
// nothing when the call is on the Once itself.
func promoted(pass *analysis.Pass, sel *ast.SelectorExpr) (fields []*types.Var) {
	selection := pass.TypesInfo.Selections[sel]
	if selection == nil {
		return nil
	}
	index, typ := selection.Index(), selection.Recv()
	for _, i := range index[:len(index)-1] {
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = ptr.Elem()
		}
		st, ok := typ.Underlying().(*types.Struct)
		if !ok {
			return nil
		}
		fields = append(fields, st.Field(i))
		typ = fields[len(fields)-1].Type()
	}
	return fields
}

// declares reports whether the loop declares the Once or binds it in a range
// clause, which leaves every iteration with a fresh one. Another loop that
// binds a pointer to an object the loop does not make does not count.
func (s scope) declares(once *types.Var, loop ast.Node) bool {
	if !encloses(loop, once.Pos()) {
		return false
	}
	for _, l := range s.loops {
		if !encloses(l, once.Pos()) || encloses(body(l), once.Pos()) {
			continue
		}
		// A range clause binds a copy each iteration. A pointer it binds
		// reaches an object that outlives the iteration unless the clause
		// belongs to the loop itself.
		r, ok := l.(*ast.RangeStmt)
		if !ok || r.Tok != token.DEFINE {
			return false
		}
		if _, pointer := once.Type().(*types.Pointer); l != loop && pointer {
			return false
		}
	}
	return true
}

// body returns the block of a loop.
func body(loop ast.Node) *ast.BlockStmt {
	switch loop := loop.(type) {
	case *ast.ForStmt:
		return loop.Body
	case *ast.RangeStmt:
		return loop.Body
	}
	return nil
}

// targets returns the expressions that lit assigns to.
func targets(lit *ast.FuncLit) (out []ast.Expr) {
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			out = append(out, n.Lhs...)
		case *ast.IncDecStmt:
			out = append(out, n.X)
		}
		return true
	})
	return out
}

// writesTarget reports whether id is a value that one of written writes rather
// than reads: seen[name] = true writes seen and reads name.
func (s scope) writesTarget(written []ast.Expr, id *ast.Ident) bool {
	obj := s.pass.TypesInfo.ObjectOf(id)
	for _, target := range written {
		if !encloses(target, id.Pos()) || writtenVar(s.pass, target) != obj {
			continue
		}
		return true
	}
	return false
}

// writtenVar returns the variable an assignment target writes, looking through
// an index, which the write reads.
func writtenVar(pass *analysis.Pass, target ast.Expr) *types.Var {
	for {
		index, ok := ast.Unparen(target).(*ast.IndexExpr)
		if !ok {
			return root(pass, target)
		}
		target = index.X
	}
}

// rebuilds reports whether the loop writes the Once or passes a pointer to it:
// either leaves the call with an Once that has not run.
func (s scope) rebuilds(recv ast.Expr, loop ast.Node, call *ast.CallExpr) bool {
	var found bool
	ast.Inspect(loop, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if s.writes(lhs, recv) {
					found = true
				}
			}
		case *ast.RangeStmt:
			// A range clause assigns without an AssignStmt.
			if n.Tok == token.ASSIGN {
				found = s.writes(n.Key, recv) || s.writes(n.Value, recv)
			}
		case *ast.DeferStmt, *ast.GoStmt:
			// A write or a call inside a deferred or a spawned function
			// does not run before the call.
			return false
		case *ast.CallExpr:
			if n != call {
				found = s.passed(n.Args, recv)
			}
		}
		return !found
	})
	return found
}

// writesField reports whether target writes a field that a promoted Do on recv
// is reached through, or a value holding one.
func (s scope) writesField(target *ast.SelectorExpr, recv ast.Expr) bool {
	if !s.promotedField(target.Sel) {
		return false
	}
	for expr := target.X; ; {
		if astexpr.Same(s.pass, expr, recv) {
			return true
		}
		sel, ok := ast.Unparen(expr).(*ast.SelectorExpr)
		if !ok {
			return false
		}
		expr = sel.X
	}
}

// promotedField reports whether id names a field that a promoted Do is reached
// through.
func (s scope) promotedField(id *ast.Ident) bool {
	obj := s.pass.TypesInfo.ObjectOf(id)
	for _, field := range s.fields {
		if obj == field {
			return true
		}
	}
	return false
}

// passed reports whether a call given args can write the Once recv names: an
// argument of pointer type lets the callee give it a new value.
func (s scope) passed(args []ast.Expr, recv ast.Expr) bool {
	for _, arg := range args {
		_, pointer := s.pass.TypesInfo.TypeOf(arg).(*types.Pointer)
		if pointer && s.writes(arg, recv) {
			return true
		}
	}
	return false
}

// writes reports whether an assignment to lhs writes the Once that recv names:
// lhs may be the Once itself, the value that holds it, a pointer to either, or
// the field a promoted Do is reached through.
func (s scope) writes(lhs, recv ast.Expr) bool {
	lhs, recv = strip(lhs), strip(recv)
	target, ok := lhs.(*ast.SelectorExpr)
	if ok && s.writesField(target, recv) {
		return true
	}
	for {
		if astexpr.Same(s.pass, lhs, recv) {
			return true
		}
		selectExpr, ok := recv.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		recv = strip(selectExpr.X)
	}
}

// strip returns e without parentheses or a leading pointer or address
// operator, so that *once and once compare as the same Once.
func strip(e ast.Expr) ast.Expr {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return ast.Unparen(e)
			}
			e = x.X
		default:
			return ast.Unparen(e)
		}
	}
}

// root returns the variable an expression's value is held in, or nil when the
// value comes from somewhere the analyzer cannot name: a call, an index, or a
// field of a value that is not a variable.
func root(pass *analysis.Pass, expr ast.Expr) *types.Var {
	for {
		switch x := strip(expr).(type) {
		case *ast.Ident:
			v, _ := pass.TypesInfo.ObjectOf(x).(*types.Var)
			return v
		case *ast.SelectorExpr:
			// A selector on a package names a variable there. Any other
			// selector reads a field or a method, so the value is traced
			// through the expression on its left.
			if pass.TypesInfo.Selections[x] == nil {
				v, _ := pass.TypesInfo.ObjectOf(x.Sel).(*types.Var)
				return v
			}
			expr = x.X
		default:
			return nil
		}
	}
}
