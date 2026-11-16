// Package nilinterfacecmp provides an analysis.Analyzer that reports a
// comparison of an interface value that holds a typed nil with nil.
//
// Assigning a nil pointer, slice, map, channel, function, or unsafe.Pointer
// to an interface stores the value's concrete type. The Go spec's Variables
// section gives the example "x = v // x has value (*T)(nil) and dynamic type
// *T", and its comparison operators section says two interface values are
// equal if they have "identical dynamic types and equal dynamic values or if
// both have value nil". The interface value has dynamic type *T, and the
// predeclared nil has none, so a comparison between them is always true for !=
// and always false for ==.
//
// The analysis tracks a nil of concrete type from the assignment or conversion
// that puts it in an interface to a comparison with nil in the same function.
// A value a call returns is not tracked.
//
// Report only: the result of the comparison is fixed, and a repair would
// change what the code tests.
//
// The README catalogs every check and exemption.
//
// The rule is adapted from staticcheck's SA4023, which also reads the nilness
// of function results.
package nilinterfacecmp

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilinterfacecmp",
	Doc:  "reports a comparison of an interface holding a typed nil with nil",
	Run:  run,
}

// nilness is what a tracked variable is known to hold.
type nilness uint8

const (
	unknown nilness = iota
	nilValue
	typedNil
)

// state maps a variable to what it is known to hold.
type state map[types.Object]nilness

// analyzer holds the facts one function body shares.
type analyzer struct {
	pass        *analysis.Pass
	lo, hi      token.Pos
	skip        map[types.Object]bool
	ranges      map[ast.Expr]bool
	selects     map[*ast.AssignStmt]bool
	commTargets map[ast.Expr][]ast.Expr
	seed        state
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			var (
				body *ast.BlockStmt
				typ  *ast.FuncType
			)
			switch n := n.(type) {
			case *ast.FuncDecl:
				body, typ = n.Body, n.Type
			case *ast.FuncLit:
				body, typ = n.Body, n.Type
			default:
				return true
			}
			if body != nil {
				var (
					sel = selectAssignments(body)
					a   = &analyzer{
						pass:        pass,
						lo:          n.Pos(),
						hi:          n.End(),
						skip:        skipObjects(pass, body),
						ranges:      rangeTargets(body),
						selects:     sel.assignments,
						commTargets: sel.targets,
						seed:        resultState(pass, typ),
					}
				)
				a.function(body)
			}
			return true
		})
	}
	return nil, nil
}

// function analyzes one function body.
func (a *analyzer) function(body *ast.BlockStmt) {
	graph := cfg.New(body, mayReturn(a.pass))
	if len(graph.Blocks) == 0 {
		return
	}
	preds := make([][]*cfg.Block, len(graph.Blocks))
	for _, b := range graph.Blocks {
		for _, succ := range b.Succs {
			preds[succ.Index] = append(preds[succ.Index], b)
		}
	}
	ins := make([]state, len(graph.Blocks))
	outs := make([]state, len(graph.Blocks))
	entry := graph.Blocks[0]
	queue := []*cfg.Block{entry}
	queued := map[*cfg.Block]bool{entry: true}
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		queued[b] = false
		var in state
		if b != entry {
			in = inState(preds[b.Index], outs)
			if in == nil {
				continue
			}
		} else {
			in = clone(a.seed)
		}
		if outs[b.Index] != nil && same(ins[b.Index], in) {
			continue
		}
		ins[b.Index] = in
		out := a.process(b, in, false)
		if same(outs[b.Index], out) {
			continue
		}
		outs[b.Index] = out
		for _, succ := range b.Succs {
			if succ.Live && !queued[succ] {
				queue = append(queue, succ)
				queued[succ] = true
			}
		}
	}
	for _, b := range graph.Blocks {
		if b.Live {
			a.process(b, ins[b.Index], true)
		}
	}
}

// inState returns what the predecessors agree on.
func inState(preds []*cfg.Block, outs []state) (in state) {
	for _, pred := range preds {
		if outs[pred.Index] == nil {
			continue
		}
		if in == nil {
			in = clone(outs[pred.Index])
			continue
		}
		out := state{}
		for obj, val := range in {
			if other, ok := outs[pred.Index][obj]; ok && other == val {
				out[obj] = val
			}
		}
		in = out
	}
	return in
}

// process walks one block in order and returns the state at its end.
func (a *analyzer) process(b *cfg.Block, in state, report bool) state {
	st := clone(in)
	for _, node := range b.Nodes {
		a.node(node, st, report)
	}
	return st
}

// node updates the state for one statement or expression the CFG holds.
func (a *analyzer) node(node ast.Node, st state, report bool) {
	switch n := node.(type) {
	case *ast.AssignStmt:
		if a.selects[n] {
			for _, rhs := range n.Rhs {
				a.expr(rhs, st, report)
			}
			return
		}
		for _, rhs := range n.Rhs {
			a.expr(rhs, st, report)
		}
		for _, lhs := range n.Lhs {
			a.expr(lhs, st, report)
		}
		a.assign(n, st)
	case *ast.ValueSpec:
		for _, value := range n.Values {
			a.expr(value, st, report)
		}
		a.declare(n, st)
	case *ast.ReturnStmt:
		for _, result := range n.Results {
			a.expr(result, st, report)
		}
	case *ast.ExprStmt:
		a.expr(n.X, st, report)
	case *ast.SendStmt:
		a.expr(n.Chan, st, report)
		a.expr(n.Value, st, report)
	case *ast.IncDecStmt:
		a.expr(n.X, st, report)
		a.set(n.X, unknown, st)
	case *ast.GoStmt:
		a.expr(n.Call, st, report)
	case *ast.DeferStmt:
		a.expr(n.Call, st, report)
	case ast.Expr:
		if a.ranges[n] {
			a.set(n, unknown, st)
			return
		}
		if lhs, ok := a.commTargets[n]; ok {
			for _, target := range lhs {
				a.set(target, unknown, st)
			}
			return
		}
		a.expr(n, st, report)
	}
}

// mayReturn reports whether a call may return. The CFG drops the edges after a
// call that cannot.
func mayReturn(pass *analysis.Pass) func(*ast.CallExpr) bool {
	return func(call *ast.CallExpr) bool {
		switch fun := ast.Unparen(call.Fun).(type) {
		case *ast.Ident:
			obj := pass.TypesInfo.ObjectOf(fun)
			if builtin, ok := obj.(*types.Builtin); ok {
				if builtin.Name() == "panic" {
					return false
				}
			}
		case *ast.SelectorExpr:
			fn, ok := pass.TypesInfo.Uses[fun.Sel].(*types.Func)
			if !ok {
				return true
			}
			switch fn.FullName() {
			case "os.Exit", "runtime.Goexit",
				"log.Fatal", "log.Fatalf", "log.Fatalln",
				"log.Panic", "log.Panicf", "log.Panicln":
				return false
			}
		}
		return true
	}
}

// expr reports comparisons in one expression. A function literal is analyzed
// as a separate function.
func (a *analyzer) expr(e ast.Expr, st state, report bool) {
	if e == nil {
		return
	}
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			if report {
				a.compare(n, st)
			}
		}
		return true
	})
}

// compare reports a comparison of a typed nil interface with the predeclared
// nil.
func (a *analyzer) compare(bin *ast.BinaryExpr, st state) {
	if bin.Op != token.EQL && bin.Op != token.NEQ {
		return
	}
	var operand ast.Expr
	switch {
	case a.isNil(bin.X):
		operand = bin.Y
	case a.isNil(bin.Y):
		operand = bin.X
	default:
		return
	}
	typ := a.pass.TypesInfo.TypeOf(operand)
	if !isInterface(typ) || a.valueOf(operand, st) != typedNil {
		return
	}
	message := "interface holds a typed nil, so the comparison is always false"
	if bin.Op == token.NEQ {
		message = "interface holds a typed nil, so the comparison is always true"
	}
	a.pass.Report(analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: message,
	})
}

// isNil reports whether e is the predeclared nil.
func (a *analyzer) isNil(e ast.Expr) bool {
	tv, ok := a.pass.TypesInfo.Types[e]
	return ok && tv.IsNil()
}

// assign updates the variables an assignment writes, all from the state before
// the assignment.
func (a *analyzer) assign(n *ast.AssignStmt, st state) {
	if n.Tok != token.DEFINE && n.Tok != token.ASSIGN {
		for _, lhs := range n.Lhs {
			a.set(lhs, unknown, st)
		}
		return
	}
	if len(n.Rhs) != len(n.Lhs) {
		for _, lhs := range n.Lhs {
			a.set(lhs, unknown, st)
		}
		return
	}
	next := make([]nilness, len(n.Lhs))
	for i, rhs := range n.Rhs {
		target := a.pass.TypesInfo.TypeOf(n.Lhs[i])
		next[i] = a.status(target, rhs, a.valueOf(rhs, st))
	}
	for i, lhs := range n.Lhs {
		a.set(lhs, next[i], st)
	}
}

// declare updates the variables a var spec declares.
func (a *analyzer) declare(spec *ast.ValueSpec, st state) {
	for i, name := range spec.Names {
		if name.Name == "_" {
			continue
		}
		obj := a.pass.TypesInfo.ObjectOf(name)
		if obj == nil {
			continue
		}
		var status nilness
		switch {
		case len(spec.Values) == 0:
			status = zeroStatus(obj.Type())
		case len(spec.Values) == len(spec.Names):
			value := a.valueOf(spec.Values[i], st)
			status = a.status(obj.Type(), spec.Values[i], value)
		default:
			status = unknown
		}
		a.put(obj, status, st)
	}
}

// set writes status for the variable lhs names.
func (a *analyzer) set(lhs ast.Expr, status nilness, st state) {
	ident, ok := ast.Unparen(lhs).(*ast.Ident)
	if !ok || ident.Name == "_" {
		return
	}
	a.put(a.pass.TypesInfo.ObjectOf(ident), status, st)
}

// put writes status for obj.
func (a *analyzer) put(obj types.Object, status nilness, st state) {
	if !a.trackable(obj) {
		delete(st, obj)
		return
	}
	if status == unknown {
		delete(st, obj)
		return
	}
	st[obj] = status
}

// trackable reports whether a variable can be followed in this function.
func (a *analyzer) trackable(obj types.Object) bool {
	if obj == nil || a.skip[obj] {
		return false
	}
	if obj.Pos() < a.lo || obj.Pos() >= a.hi {
		return false // a package-level variable or one a literal captured
	}
	return true
}

// status returns what target holds when rhs, whose value is val, is assigned.
func (a *analyzer) status(target types.Type, rhs ast.Expr, val nilness) nilness {
	if target == nil || isTypeParam(target) {
		return unknown
	}
	if isInterface(target) {
		switch val {
		case typedNil:
			return typedNil
		case nilValue:
			value := a.pass.TypesInfo.TypeOf(rhs)
			switch {
			case isUntypedNil(value) || isInterface(value):
				return nilValue
			case isNilable(value):
				return typedNil
			}
		}
		return unknown
	}
	if isNilable(target) && val == nilValue {
		return nilValue
	}
	return unknown
}

// valueOf returns what e is known to hold.
func (a *analyzer) valueOf(e ast.Expr, st state) nilness {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if a.isNil(e) {
			return nilValue
		}
		obj := a.pass.TypesInfo.ObjectOf(e)
		if !a.trackable(obj) {
			return unknown
		}
		return st[obj]
	case *ast.CallExpr:
		if len(e.Args) != 1 || !a.conversion(e) {
			return unknown
		}
		return a.status(
			a.pass.TypesInfo.TypeOf(e),
			e.Args[0],
			a.valueOf(e.Args[0], st),
		)
	}
	return unknown
}

// conversion reports whether call converts a value.
func (a *analyzer) conversion(call *ast.CallExpr) bool {
	tv, ok := a.pass.TypesInfo.Types[ast.Unparen(call.Fun)]
	return ok && tv.IsType()
}

// resultState returns the named results of a function, which start with their
// zero values.
func resultState(pass *analysis.Pass, typ *ast.FuncType) state {
	st := state{}
	if typ == nil || typ.Results == nil {
		return st
	}
	for _, field := range typ.Results.List {
		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}
			obj := pass.TypesInfo.ObjectOf(name)
			if obj == nil {
				continue
			}
			if status := zeroStatus(obj.Type()); status != unknown {
				st[obj] = status
			}
		}
	}
	return st
}

// zeroStatus returns what an uninitialized variable of type typ holds.
func zeroStatus(typ types.Type) nilness {
	if isTypeParam(typ) {
		return unknown
	}
	if isInterface(typ) || isNilable(typ) {
		return nilValue
	}
	return unknown
}

// isNilable reports whether typ is a type whose values can be nil.
func isNilable(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch typ := types.Unalias(typ).Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan,
		*types.Signature:
		return true
	case *types.Basic:
		return typ.Kind() == types.UnsafePointer ||
			typ.Kind() == types.UntypedNil
	}
	return false
}

// isUntypedNil reports whether typ is the type of the predeclared nil.
func isUntypedNil(typ types.Type) bool {
	if typ == nil {
		return false
	}
	basic, ok := types.Unalias(typ).(*types.Basic)
	return ok && basic.Kind() == types.UntypedNil
}

// isInterface reports whether typ is an interface type other than a type
// parameter.
func isInterface(typ types.Type) bool {
	if typ == nil || isTypeParam(typ) {
		return false
	}
	return types.IsInterface(types.Unalias(typ))
}

// isTypeParam reports whether typ is a type parameter.
func isTypeParam(typ types.Type) bool {
	if typ == nil {
		return false
	}
	_, ok := types.Unalias(typ).(*types.TypeParam)
	return ok
}

// selection is what a select hoists before itself.
type selection struct {
	assignments map[*ast.AssignStmt]bool
	targets     map[ast.Expr][]ast.Expr
}

// selectAssignments returns the assignments a select hoists before itself and
// the left sides each selected case assigns. The hoisted assignments do not
// run until a case is selected.
func selectAssignments(body *ast.BlockStmt) selection {
	out := selection{
		assignments: make(map[*ast.AssignStmt]bool),
		targets:     make(map[ast.Expr][]ast.Expr),
	}
	ast.Inspect(body, func(n ast.Node) bool {
		stmt, ok := n.(*ast.SelectStmt)
		if !ok {
			return true
		}
		for _, clause := range stmt.Body.List {
			assign, ok := clause.(*ast.CommClause).Comm.(*ast.AssignStmt)
			if !ok {
				continue
			}
			out.assignments[assign] = true
			if len(assign.Lhs) > 0 {
				out.targets[assign.Lhs[0]] = assign.Lhs
			}
		}
		return true
	})
	return out
}

// rangeTargets returns the key and value expressions a range statement assigns
// with =. The CFG adds them as bare expressions, so their assignments are
// tracked here.
func rangeTargets(body *ast.BlockStmt) map[ast.Expr]bool {
	out := make(map[ast.Expr]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		r, ok := n.(*ast.RangeStmt)
		if !ok || r.Tok != token.ASSIGN {
			return true
		}
		if r.Key != nil {
			out[r.Key] = true
		}
		if r.Value != nil {
			out[r.Value] = true
		}
		return true
	})
	return out
}

// skipObjects returns the variables this body should not follow: one whose
// address is taken, and one a function literal writes.
func skipObjects(pass *analysis.Pass, body *ast.BlockStmt) map[types.Object]bool {
	skip := make(map[types.Object]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				mark(pass, n.X, skip)
			}
		case *ast.SelectorExpr:
			if implicitAddress(pass, n) {
				mark(pass, n.X, skip)
			}
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		literal, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		ast.Inspect(literal.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					mark(pass, lhs, skip)
				}
			case *ast.IncDecStmt:
				mark(pass, n.X, skip)
			case *ast.RangeStmt:
				if n.Tok == token.ASSIGN {
					mark(pass, n.Key, skip)
					mark(pass, n.Value, skip)
				}
			}
			return true
		})
		return false
	})
	return skip
}

// implicitAddress reports whether selecting a pointer-receiver method takes
// the address of the value before it.
func implicitAddress(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return false
	}
	typ := pass.TypesInfo.TypeOf(sel.X)
	if typ == nil || isInterface(typ) {
		return false
	}
	if _, ok := types.Unalias(typ).(*types.Pointer); ok {
		return false
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	_, ok = types.Unalias(sig.Recv().Type()).(*types.Pointer)
	return ok
}

// mark records the variable e names.
func mark(pass *analysis.Pass, e ast.Expr, skip map[types.Object]bool) {
	if e == nil {
		return
	}
	ident, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || ident.Name == "_" {
		return
	}
	if obj := pass.TypesInfo.ObjectOf(ident); obj != nil {
		skip[obj] = true
	}
}

// clone returns a copy of st.
func clone(st state) state {
	out := make(state, len(st))
	maps.Copy(out, st)
	return out
}

// same reports whether two states hold the same variables and values.
func same(x, y state) bool {
	if (x == nil) != (y == nil) || len(x) != len(y) {
		return false
	}
	for obj, val := range x {
		if y[obj] != val {
			return false
		}
	}
	return true
}
