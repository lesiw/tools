// Package defermisuse provides an analysis.Analyzer that reports a recover
// call or a return from the wrong function.
//
// The recover builtin stops a panic only when a deferred function calls it
// directly. A recover call directly in the body of a function that is not the
// deferred function does nothing. A return inside a deferred literal ends the
// literal, and its results are discarded; it does not return from the function
// that deferred it.
//
// Report only: moving recover into the deferred function needs that function,
// and removing a return changes what the literal does after it. Only the
// author knows which repair is meant.
//
// Test files are not analyzed. An exported function is not checked, because
// another package can defer it. A function literal stored as a value, or a
// named function that the package does not use or uses as a value, is not
// checked, because a caller can defer it. A main or init function is checked,
// because the runtime calls it and a recover in that invocation does nothing.
//
// The rule is adapted from revive's defer, covering its recover,
// immediate-recover, and return parts. The loop case belongs to deferinloop,
// and the method-call and call-chain cases are not checked. The README
// catalogs the checks and exemptions.
package defermisuse

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "defermisuse",
	Doc:  "reports a recover call or a return from the wrong function",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	deferrals := collect(pass)
	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		ast.Walk(visitor{pass: pass, deferrals: deferrals}, file)
	}
	return nil, nil
}

// deferrals holds what the package does with its functions.
type deferrals struct {
	// funcs run as deferred functions, and lits are the literals deferred
	// through a variable.
	funcs map[types.Object]struct{}
	lits  map[*ast.FuncLit]struct{}

	// used and valueUsed track the named functions this package refers to.
	used      map[types.Object]bool
	valueUsed map[types.Object]bool

	// literalValues holds the literals stored as values, which a caller can
	// defer.
	literalValues map[*ast.FuncLit]bool
}

type visitor struct {
	pass      *analysis.Pass
	deferrals deferrals

	// funcObj is the named function whose body holds the node, when one does.
	funcObj types.Object

	// registration holds while an argument or the function value of a defer
	// is evaluated, where a recover call runs at the defer statement.
	registration bool

	// inDeferredFunc holds directly inside the body of the deferred function
	// literal, where recover works and a return affects only the literal.
	inDeferredFunc bool

	// mayBeDeferred holds inside a literal stored as a value, which a caller
	// can defer.
	mayBeDeferred bool
}

func (v visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.DeferStmt:
		v.deferStmt(n)
		return nil
	case *ast.FuncDecl:
		return visitor{
			pass:      v.pass,
			deferrals: v.deferrals,
			funcObj:   origin(v.pass.TypesInfo.Defs[n.Name]),
		}
	case *ast.FuncLit:
		if _, ok := v.deferrals.lits[n]; ok {
			ast.Walk(visitor{
				pass:           v.pass,
				deferrals:      v.deferrals,
				inDeferredFunc: true,
			}, n.Body)
			return nil
		}
		if v.deferrals.literalValues[n] {
			ast.Walk(visitor{
				pass:          v.pass,
				deferrals:     v.deferrals,
				mayBeDeferred: true,
			}, n.Body)
			return nil
		}
		// A literal reached here is not the deferred function itself.
		return visitor{pass: v.pass, deferrals: v.deferrals}
	case *ast.GoStmt:
		v.goStmt(n)
		return nil
	case *ast.CallExpr:
		if isRecover(v.pass, n) && !v.recoverMayWork() {
			if v.registration {
				v.pass.Reportf(n.Pos(), "recover runs when the defer is registered; call it inside the deferred function")
			} else {
				v.pass.Reportf(n.Pos(), "recover outside a deferred function has no effect")
			}
		}
		return v
	case *ast.ReturnStmt:
		if v.inDeferredFunc && len(n.Results) != 0 {
			v.pass.Reportf(n.Pos(), "return in a deferred function does not return from the function that deferred it")
		}
		return v
	}
	return v
}

// deferStmt checks the deferred call and walks its parts with the context each
// one runs in.
func (v visitor) deferStmt(d *ast.DeferStmt) {
	fun := unparen(d.Call.Fun)
	if isRecover(v.pass, d.Call) {
		v.pass.Reportf(d.Pos(), "recover is the deferred call; call it directly in a deferred function")
	}
	if lit, ok := fun.(*ast.FuncLit); ok {
		ast.Walk(visitor{
			pass:           v.pass,
			deferrals:      v.deferrals,
			inDeferredFunc: true,
		}, lit.Body)
	} else {
		ast.Walk(visitor{
			pass:           v.pass,
			deferrals:      v.deferrals,
			funcObj:        v.funcObj,
			registration:   true,
			inDeferredFunc: v.inDeferredFunc,
			mayBeDeferred:  v.mayBeDeferred,
		}, d.Call.Fun)
	}
	for _, arg := range d.Call.Args {
		ast.Walk(visitor{
			pass:           v.pass,
			deferrals:      v.deferrals,
			funcObj:        v.funcObj,
			registration:   true,
			inDeferredFunc: v.inDeferredFunc,
			mayBeDeferred:  v.mayBeDeferred,
		}, arg)
	}
}

// goStmt checks the call that starts a goroutine. The function value and the
// arguments are evaluated in this goroutine, and the call runs in another one.
func (v visitor) goStmt(g *ast.GoStmt) {
	if isRecover(v.pass, g.Call) {
		v.pass.Reportf(g.Pos(), "recover in a new goroutine has no effect")
		return
	}
	ast.Walk(v, g.Call.Fun)
	for _, arg := range g.Call.Args {
		ast.Walk(v, arg)
	}
}

// recoverMayWork reports whether a recover call at the visitor's position may
// run directly in a deferred function.
func (v visitor) recoverMayWork() bool {
	return v.inDeferredFunc || v.mayBeDeferred ||
		v.isDeferredFunc() || !v.funcCanReport()
}

// isDeferredFunc reports whether the visitor sits in a named function that
// runs as a deferred function.
func (v visitor) isDeferredFunc() bool {
	_, ok := v.deferrals.funcs[v.funcObj]
	return ok
}

// funcCanReport reports whether the visitor is outside a named function, or
// inside an unexported function that this package uses directly and never as
// a value.
func (v visitor) funcCanReport() bool {
	if v.funcObj == nil {
		return true
	}
	if v.funcObj.Exported() {
		return false
	}
	if !v.deferrals.used[v.funcObj] && !isEntry(v.pass, v.funcObj) {
		return false
	}
	return !v.deferrals.valueUsed[v.funcObj]
}

// isEntry reports whether fn is called by the runtime, where recover does not
// run in a deferred function.
func isEntry(pass *analysis.Pass, fn types.Object) bool {
	f, ok := fn.(*types.Func)
	if !ok {
		return false
	}
	sig, ok := f.Type().(*types.Signature)
	if !ok || sig.Recv() != nil {
		return false
	}
	if f.Name() == "init" {
		return true
	}
	return f.Name() == "main" && pass.Pkg.Name() == "main"
}

// collect gathers what the package does with its functions.
func collect(pass *analysis.Pass) deferrals {
	var (
		out = deferrals{
			funcs:         make(map[types.Object]struct{}),
			lits:          make(map[*ast.FuncLit]struct{}),
			used:          make(map[types.Object]bool),
			valueUsed:     make(map[types.Object]bool),
			literalValues: make(map[*ast.FuncLit]bool),
		}
		methods = make(map[string][]*types.Func)
		values  = make(map[types.Object][]assignment)
	)

	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		collectMethods(pass, file, methods)
		collectUses(pass, file, &out)
		collectAssignments(pass, file, values)
	}
	propagateInterfaceUses(&out, methods)

	for _, file := range pass.Files {
		if isTestFile(pass, file) {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			d, ok := n.(*ast.DeferStmt)
			if !ok {
				return true
			}
			addDeferred(pass, d.Call.Fun, &out, methods)
			markDeferredLiteral(pass, d.Call.Fun, d.Pos(), values, &out)
			return true
		})
	}
	return out
}

// assignment is one value stored in a variable.
type assignment struct {
	lit *ast.FuncLit
	pos token.Pos
}

// collectAssignments records the literals stored directly in variables.
func collectAssignments(pass *analysis.Pass, file *ast.File, values map[types.Object][]assignment) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return true
			}
			for i := range n.Lhs {
				recordValue(pass, n.Lhs[i], n.Rhs[i], values)
			}
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if i < len(n.Values) {
					recordValue(pass, name, n.Values[i], values)
				}
			}
		}
		return true
	})
}

// recordValue records one assignment to a variable.
func recordValue(pass *analysis.Pass, lhs, rhs ast.Expr, values map[types.Object][]assignment) {
	id, ok := unparen(lhs).(*ast.Ident)
	if !ok {
		return
	}
	obj := pass.TypesInfo.ObjectOf(id)
	if obj == nil {
		return
	}
	lit, _ := unparen(rhs).(*ast.FuncLit)
	values[obj] = append(values[obj], assignment{lit: lit, pos: rhs.Pos()})
}

// markDeferredLiteral records the literal that a defer of a variable runs.
func markDeferredLiteral(pass *analysis.Pass, e ast.Expr, pos token.Pos, values map[types.Object][]assignment, out *deferrals) {
	var (
		obj  = callee(pass, e)
		last *assignment
	)
	for i := range values[obj] {
		if values[obj][i].pos < pos {
			last = &values[obj][i]
		}
	}
	if last == nil || last.lit == nil {
		return
	}
	out.lits[last.lit] = struct{}{}
}

// collectMethods records the methods declared in file.
func collectMethods(pass *analysis.Pass, file *ast.File, methods map[string][]*types.Func) {
	ast.Inspect(file, func(n ast.Node) bool {
		f, ok := n.(*ast.FuncDecl)
		if !ok || f.Recv == nil {
			return true
		}
		obj := origin(pass.TypesInfo.Defs[f.Name])
		if fn, ok := obj.(*types.Func); ok {
			methods[f.Name.Name] = append(methods[f.Name.Name], fn)
		}
		return true
	})
}

// collectUses records how file refers to each function.
func collectUses(pass *analysis.Pass, file *ast.File, out *deferrals) {
	parents := astexpr.Parents(file)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			if !directlyCalled(n, parents) {
				out.literalValues[n] = true
			}
		case *ast.SelectorExpr:
			fn, ok := origin(pass.TypesInfo.ObjectOf(n.Sel)).(*types.Func)
			if !ok {
				return true
			}
			out.used[fn] = true
			if !directlyCalled(n, parents) {
				out.valueUsed[fn] = true
			}
		case *ast.Ident:
			if sel, ok := parents[n].(*ast.SelectorExpr); ok && sel.Sel == n {
				return true
			}
			fn, ok := origin(pass.TypesInfo.Uses[n]).(*types.Func)
			if !ok {
				return true
			}
			out.used[fn] = true
			if !directlyCalled(n, parents) {
				out.valueUsed[fn] = true
			}
		}
		return true
	})
}

// directlyCalled reports whether e is the function of a call.
func directlyCalled(e ast.Expr, parents map[ast.Node]ast.Node) bool {
	for {
		switch p := parents[e].(type) {
		case *ast.ParenExpr:
			if p.X != e {
				return false
			}
			e = p
		case *ast.IndexExpr:
			if p.X != e {
				return false
			}
			e = p
		case *ast.IndexListExpr:
			if p.X != e {
				return false
			}
			e = p
		case *ast.CallExpr:
			return p.Fun == e
		default:
			return false
		}
	}
}

// addDeferred records the function that e names as deferred.
func addDeferred(pass *analysis.Pass, e ast.Expr, out *deferrals, methods map[string][]*types.Func) {
	fn, ok := callee(pass, e).(*types.Func)
	if !ok {
		return
	}
	out.funcs[fn] = struct{}{}
	if isInterfaceMethod(fn) {
		markInterfaceMethods(fn, out, methods)
	}
}

// propagateInterfaceUses records the concrete methods that an interface call
// can run. A direct call with more than one implementation is left alone,
// because the analyzer cannot tell which method runs.
func propagateInterfaceUses(out *deferrals, methods map[string][]*types.Func) {
	for fn := range out.used {
		f, ok := fn.(*types.Func)
		if !ok {
			continue
		}
		iface := interfaceOf(f)
		if iface == nil {
			continue
		}
		var candidates []*types.Func
		for _, method := range methods[f.Name()] {
			sig, ok := method.Type().(*types.Signature)
			if !ok || sig.Recv() == nil {
				continue
			}
			if implementsInterface(iface, sig.Recv().Type()) {
				candidates = append(candidates, method)
			}
		}
		if out.valueUsed[f] {
			for _, method := range candidates {
				out.used[method] = true
				out.valueUsed[method] = true
			}
			continue
		}
		if len(candidates) == 1 {
			out.used[candidates[0]] = true
		}
	}
}

// markInterfaceMethods records the methods that can run when fn is deferred.
func markInterfaceMethods(fn *types.Func, out *deferrals, methods map[string][]*types.Func) {
	iface := interfaceOf(fn)
	if iface == nil {
		return
	}
	for _, method := range methods[fn.Name()] {
		if !matchesInterface(fn, method) {
			continue
		}
		sig, ok := method.Type().(*types.Signature)
		if !ok || sig.Recv() == nil {
			continue
		}
		if implementsInterface(iface, sig.Recv().Type()) {
			out.funcs[method] = struct{}{}
		}
	}
}

// interfaceOf returns the interface that fn belongs to.
func interfaceOf(fn *types.Func) *types.Interface {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	iface, ok := sig.Recv().Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	return iface
}

// implementsInterface reports whether recv has every method of iface.
func implementsInterface(iface *types.Interface, recv types.Type) bool {
	for want := range iface.Methods() {
		var (
			obj, _, _ = types.LookupFieldOrMethod(
				recv, true, want.Pkg(),
				want.Name(),
			)
			got, ok = obj.(*types.Func)
		)
		if !ok {
			return false
		}
		x, ok := want.Type().(*types.Signature)
		if !ok {
			return false
		}
		y, ok := got.Type().(*types.Signature)
		if !ok {
			return false
		}
		if !sameSignature(x, y) && !sameSignatureWildcard(x, y) {
			return false
		}
	}
	return true
}

// matchesInterface reports whether method can run when iface is deferred.
func matchesInterface(iface, method *types.Func) bool {
	if iface.Name() != method.Name() {
		return false
	}
	x, ok := iface.Type().(*types.Signature)
	if !ok {
		return false
	}
	y, ok := method.Type().(*types.Signature)
	if !ok {
		return false
	}
	return sameSignature(x, y) || sameSignatureWildcard(x, y)
}

// sameSignature reports whether x and y have the same parameters and results,
// ignoring their receivers.
func sameSignature(x, y *types.Signature) bool {
	if x.Variadic() != y.Variadic() {
		return false
	}
	if !sameTuple(x.Params(), y.Params()) {
		return false
	}
	return sameTuple(x.Results(), y.Results())
}

// sameTuple reports whether x and y hold the same types.
func sameTuple(x, y *types.Tuple) bool {
	if x.Len() != y.Len() {
		return false
	}
	for i := 0; i < x.Len(); i++ {
		if !types.Identical(x.At(i).Type(), y.At(i).Type()) {
			return false
		}
	}
	return true
}

// sameSignatureWildcard reports whether x and y have the same shape, with a
// type parameter matching any type.
func sameSignatureWildcard(x, y *types.Signature) bool {
	if x.Variadic() != y.Variadic() {
		return false
	}
	if !sameTupleWildcard(x.Params(), y.Params()) {
		return false
	}
	return sameTupleWildcard(x.Results(), y.Results())
}

// sameTupleWildcard reports whether x and y have the same shape.
func sameTupleWildcard(x, y *types.Tuple) bool {
	if x.Len() != y.Len() {
		return false
	}
	for i := 0; i < x.Len(); i++ {
		if !sameTypeWildcard(x.At(i).Type(), y.At(i).Type()) {
			return false
		}
	}
	return true
}

// sameTypeWildcard reports whether x and y have the same shape, with a type
// parameter matching any type.
func sameTypeWildcard(x, y types.Type) bool {
	if types.Identical(x, y) {
		return true
	}
	if _, ok := x.(*types.TypeParam); ok {
		return true
	}
	if _, ok := y.(*types.TypeParam); ok {
		return true
	}
	x, y = unalias(x), unalias(y)
	return sameContainerWildcard(x, y) || sameNamedWildcard(x, y) ||
		sameCallableWildcard(x, y)
}

// unalias returns t without any enclosing alias.
func unalias(t types.Type) types.Type {
	for {
		alias, ok := t.(*types.Alias)
		if !ok {
			return t
		}
		t = types.Unalias(alias)
	}
}

// sameContainerWildcard reports whether x and y are the same container with
// the same shape.
func sameContainerWildcard(x, y types.Type) bool {
	switch x := x.(type) {
	case *types.Pointer:
		y, ok := y.(*types.Pointer)
		return ok && sameTypeWildcard(x.Elem(), y.Elem())
	case *types.Slice:
		y, ok := y.(*types.Slice)
		return ok && sameTypeWildcard(x.Elem(), y.Elem())
	case *types.Array:
		y, ok := y.(*types.Array)
		return ok && x.Len() == y.Len() && sameTypeWildcard(x.Elem(), y.Elem())
	case *types.Chan:
		y, ok := y.(*types.Chan)
		return ok && x.Dir() == y.Dir() && sameTypeWildcard(x.Elem(), y.Elem())
	case *types.Map:
		y, ok := y.(*types.Map)
		return ok && sameTypeWildcard(x.Key(), y.Key()) &&
			sameTypeWildcard(x.Elem(), y.Elem())
	}
	return false
}

// sameNamedWildcard reports whether x and y are the same named type or
// struct with the same shape.
func sameNamedWildcard(x, y types.Type) bool {
	switch x := x.(type) {
	case *types.Named:
		y, ok := y.(*types.Named)
		return ok && x.Obj() == y.Obj() && sameTypesWildcard(
			x.TypeArgs(),
			y.TypeArgs(),
		)
	case *types.Struct:
		return sameStructWildcard(x, y)
	}
	return false
}

// sameStructWildcard reports whether x and y are structs with the same shape.
func sameStructWildcard(x *types.Struct, y types.Type) bool {
	s, ok := y.(*types.Struct)
	if !ok || x.NumFields() != s.NumFields() {
		return false
	}
	for i := 0; i < x.NumFields(); i++ {
		if x.Field(i).Name() != s.Field(i).Name() {
			return false
		}
		if !sameTypeWildcard(x.Field(i).Type(), s.Field(i).Type()) {
			return false
		}
	}
	return true
}

// sameCallableWildcard reports whether x and y are the same callable or
// interface with the same shape.
func sameCallableWildcard(x, y types.Type) bool {
	switch x := x.(type) {
	case *types.Interface:
		y, ok := y.(*types.Interface)
		return ok && sameInterfaceWildcard(x, y)
	case *types.Signature:
		y, ok := y.(*types.Signature)
		return ok && sameSignatureWildcard(x, y)
	case *types.Tuple:
		y, ok := y.(*types.Tuple)
		return ok && sameTupleWildcard(x, y)
	}
	return false
}

// sameTypesWildcard reports whether x and y have the same shape.
func sameTypesWildcard(x, y *types.TypeList) bool {
	if x.Len() != y.Len() {
		return false
	}
	for i := 0; i < x.Len(); i++ {
		if !sameTypeWildcard(x.At(i), y.At(i)) {
			return false
		}
	}
	return true
}

// sameInterfaceWildcard reports whether x and y have the same methods.
func sameInterfaceWildcard(x, y *types.Interface) bool {
	if x.NumExplicitMethods() != y.NumExplicitMethods() {
		return false
	}
	for method := range x.ExplicitMethods() {
		var (
			other, _, _ = types.LookupFieldOrMethod(
				y, false, method.Pkg(),
				method.Name(),
			)
			fn, ok = other.(*types.Func)
		)
		if !ok {
			return false
		}
		first, ok := method.Type().(*types.Signature)
		if !ok {
			return false
		}
		second, ok := fn.Type().(*types.Signature)
		if !ok || !sameSignatureWildcard(first, second) {
			return false
		}
	}
	return true
}

// isInterfaceMethod reports whether fn belongs to an interface.
func isInterfaceMethod(fn *types.Func) bool {
	return interfaceOf(fn) != nil
}

// isTestFile reports whether file is a test file.
func isTestFile(pass *analysis.Pass, file *ast.File) bool {
	name := pass.Fset.Position(file.Pos()).Filename
	return strings.HasSuffix(name, "_test.go")
}

// callee returns the function or method that e names.
func callee(pass *analysis.Pass, e ast.Expr) types.Object {
	switch e := unparen(e).(type) {
	case *ast.Ident:
		return origin(pass.TypesInfo.Uses[e])
	case *ast.SelectorExpr:
		return origin(pass.TypesInfo.ObjectOf(e.Sel))
	case *ast.IndexExpr:
		return callee(pass, e.X)
	case *ast.IndexListExpr:
		return callee(pass, e.X)
	}
	return nil
}

// origin returns the declared function for an instantiated generic method.
func origin(obj types.Object) types.Object {
	if fn, ok := obj.(*types.Func); ok {
		return fn.Origin()
	}
	return obj
}

// isRecover reports whether call invokes the recover builtin.
func isRecover(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := unparen(call.Fun).(*ast.Ident)
	if !ok || id.Name != "recover" {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	return ok && builtin.Name() == "recover"
}

// unparen returns e without any enclosing parentheses.
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}
