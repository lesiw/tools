// Package maplenloop provides an analysis.Analyzer that reports a call to len
// on the map a range statement iterates.
//
// The section of the spec on map types says the number of map elements is the
// map's length, and that the length "may change during execution"
// (https://go.dev/ref/spec#Map_types). The section on the range clause says
// the clause iterates through all entries of a map
// (https://go.dev/ref/spec#For_range). A loop body that leaves the map alone
// therefore reads the same length at every pass, and a call before the loop
// reads the same number.
//
// The loop must leave the map alone, and the map must be one the function
// built or one a caller passed or the function returns. A package-level map,
// and a local copy of one, are left alone, since another function may change
// them. The README lists every case.
//
// The fix declares the length above the loop and replaces the call with the
// name. It is withheld where the edit would lose a comment, jump a goto over
// the declaration, leave the loop without a line of its own, or write a line
// past the width limit.
package maplenloop

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "maplenloop",
	Doc:  "reports a call to len on the map a range statement iterates",
	Run:  run,
}

// call is one len call this rule reports: the call, the loop it sits in, the
// map it measures, and the fix the loop carries.
type call struct {
	expr   *ast.CallExpr
	loop   *ast.RangeStmt
	mapx   ast.Expr
	anchor ast.Node
	name   string // the name the fix uses
	fix    bool   // the call has a fix
}

// fixer plans the fixes for one file.
type fixer struct {
	pass   *analysis.Pass
	src    []byte
	parent map[ast.Node]ast.Node
	taken  map[ast.Node]map[string]bool
}

// run reports the calls in each file.
func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if err := checkFile(pass, file); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// checkFile reports the calls in one file, with a fix where one applies.
func checkFile(pass *analysis.Pass, file *ast.File) error {
	var (
		parent = astexpr.Parents(file)
		found  = calls(pass, parent, file)
	)
	if len(found) == 0 {
		return nil
	}
	src, err := pass.ReadFile(pass.Fset.File(file.Pos()).Name())
	if err != nil {
		return err
	}
	fix := &fixer{
		pass:   pass,
		src:    src,
		parent: parent,
		taken:  make(map[ast.Node]map[string]bool),
	}
	groups := make(map[*ast.RangeStmt][]*call)
	var order []*ast.RangeStmt
	for _, c := range found {
		if !fixable(pass, file, src, parent, c) {
			continue
		}
		if _, ok := groups[c.loop]; !ok {
			order = append(order, c.loop)
		}
		groups[c.loop] = append(groups[c.loop], c)
	}
	fixes := make(map[*ast.RangeStmt]analysis.SuggestedFix, len(order))
	for _, loop := range order {
		var (
			cs      = groups[loop]
			name, _ = fix.name(cs)
		)
		if name == "" {
			continue
		}
		for _, c := range cs {
			c.name = name
			c.fix = true
		}
		fixes[loop] = fix.hoist(cs)
	}
	for _, c := range found {
		d := diagnostic(c)
		if c.fix {
			d.SuggestedFixes = []analysis.SuggestedFix{fixes[c.loop]}
		}
		pass.Report(d)
	}
	return nil
}

// name returns the name the fix for one loop declares, and whether the fix can
// be written at all.
func (f *fixer) name(cs []*call) (string, bool) {
	var (
		first = cs[0]
		owner = f.parent[first.anchor]
		name  = f.pick(owner, cs)
	)
	if !fits(f.pass, f.src, cs, name) {
		f.release(name, owner)
		return "", false
	}
	return name, true
}

// function returns the innermost function that holds node.
func function(parent map[ast.Node]ast.Node, node ast.Node) ast.Node {
	for n := node; n != nil; n = parent[n] {
		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return n
		}
	}
	return nil
}

// shadowed reports whether name already resolves to something at one of the
// calls, where the replacement would read that instead.
func (f *fixer) shadowed(name string, cs []*call) bool {
	for _, c := range cs {
		var (
			pos   = c.expr.Pos()
			scope = f.pass.Pkg.Scope().Innermost(pos)
		)
		if scope == nil {
			continue
		}
		if _, obj := scope.LookupParent(name, pos); obj != nil {
			return true
		}
	}
	return false
}

// release drops a name that pick took for a fix that will not be written.
func (f *fixer) release(name string, block ast.Node) {
	delete(f.taken[function(f.parent, block)], name)
}

// pick returns a name for the length that nothing in the block holds, that no
// earlier fix took there, and that no declaration shadows at a call.
func (f *fixer) pick(block ast.Node, cs []*call) string {
	var (
		_, end = span(block)
		scope  = declareScope(f.pass, block)
		owner  = function(f.parent, block)
	)
	if f.taken[owner] == nil {
		f.taken[owner] = make(map[string]bool)
	}
	for i := 1; ; i++ {
		name := "length"
		if i > 1 {
			name += strconv.Itoa(i)
		}
		if f.taken[owner][name] || f.shadowed(name, cs) {
			continue
		}
		if scope != nil {
			if _, obj := scope.LookupParent(name, end-1); obj != nil {
				continue
			}
		}
		f.taken[owner][name] = true
		return name
	}
}

// calls returns the reported calls in file, in source order.
func calls(pass *analysis.Pass, parent map[ast.Node]ast.Node, file *ast.File) []*call {
	var found []*call
	ast.Inspect(file, func(n ast.Node) bool {
		expr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		mapx, ok := lengthOf(pass, expr)
		if !ok || !stable(pass, mapx) {
			return true
		}
		loop, ok := rangedOver(pass, parent, expr, mapx)
		if !ok || !proven(pass, parent, loop, mapx) {
			return true
		}
		if !invariant(pass, parent, loop, mapx) {
			return true
		}
		found = append(found, &call{
			expr:   expr,
			loop:   loop,
			mapx:   mapx,
			anchor: anchorOf(parent, loop),
		})
		return true
	})
	return found
}

// proven reports whether the function can account for every name the map has:
// it built the map or a caller passed it, and no call carried it anywhere else
// where a callee can reach it.
func proven(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, mapx ast.Expr) bool {
	if packageMap(pass, mapx) || published(pass, parent, loop, mapx) {
		return false
	}
	if parameter(pass, parent, loop, mapx) {
		return true
	}
	return made(pass, outerBody(parent, loop), mapx)
}

// published reports whether a call anywhere in the function hands over the
// map, which the callee can keep where a later call finds it.
func published(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, mapx ast.Expr) bool {
	body := outerBody(parent, loop)
	if body == nil {
		return false
	}
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !keeper(pass, call) {
			return !found
		}
		found = reaches(
			pass, parent, loop, call, base(mapx),
			pass.TypesInfo.TypeOf(mapx),
		)
		return !found
	})
	return found
}

// keeper reports whether call can keep a value it is given: the callee is not
// a builtin, a conversion, or a literal called in place.
func keeper(pass *analysis.Pass, call *ast.CallExpr) bool {
	if _, ok := ast.Unparen(call.Fun).(*ast.FuncLit); ok {
		return false
	}
	if astexpr.Pure(pass, call) {
		return false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return true
	}
	_, builtin := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return !builtin
}

// lengthOf returns the operand of call when it is a call to the builtin len
// applied to a map.
func lengthOf(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	if len(call.Args) != 1 {
		return nil, false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return nil, false
	}
	arg := ast.Unparen(call.Args[0])
	if !isMap(pass.TypesInfo.TypeOf(arg)) {
		return nil, false
	}
	return arg, true
}

// isMap reports whether t is a map type.
func isMap(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Map)
	return ok
}

// rangedOver returns the outermost range statement that iterates mapx and
// holds node, so the declaration the fix writes sits outside every loop over
// the map. It reports false for a node outside such a body, and for one inside
// a function literal, which can outlive the loop that contains it.
func rangedOver(pass *analysis.Pass, parent map[ast.Node]ast.Node, node ast.Node, mapx ast.Expr) (*ast.RangeStmt, bool) {
	var loop *ast.RangeStmt
	for n := parent[node]; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncLit:
			return loop, loop != nil
		case *ast.RangeStmt:
			if within(n.Body, node) && astexpr.Same(pass, n.X, mapx) {
				loop = n
			}
		}
	}
	return loop, loop != nil
}

// stable reports whether mapx has the form the fix reads once above the loop:
// a name, or a path of fields that does not go through a pointer.
func stable(pass *analysis.Pass, mapx ast.Expr) bool {
	switch mapx := ast.Unparen(mapx).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		if sel, ok := pass.TypesInfo.Selections[mapx]; ok {
			if sel.Kind() != types.FieldVal {
				return false
			}
		}
		if !astexpr.Follows(pass, mapx) {
			return false
		}
		return stable(pass, mapx.X)
	}
	return false
}

// packageMap reports whether mapx is a package-level value, which a call the
// analyzer cannot see can change.
func packageMap(pass *analysis.Pass, mapx ast.Expr) bool {
	ident, ok := base(mapx).(*ast.Ident)
	if !ok {
		return false
	}
	if _, ok := pass.TypesInfo.Uses[ident].(*types.PkgName); ok {
		return true
	}
	variable, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	return ok && variable.Parent() == pass.Pkg.Scope()
}

// base returns the name at the left of e: the b in b.v, the p in *p, the d in
// d[k].
func base(e ast.Expr) ast.Expr {
	for {
		var next ast.Expr
		switch e := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			next = e.X
		case *ast.IndexExpr:
			next = e.X
		case *ast.StarExpr:
			next = e.X
		default:
			return ast.Unparen(e)
		}
		e = next
	}
}

// invariant reports whether the loop leaves the map alone: nothing in its body
// changes it, the function binds no second name to it, and no goto after the
// loop runs it again.
func invariant(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop *ast.RangeStmt, mapx ast.Expr) bool {
	var (
		root = base(mapx)
		typ  = pass.TypesInfo.TypeOf(mapx)
	)
	if loop.Tok == token.ASSIGN {
		if writes(pass, parent, loop, loop.Key, root, typ) {
			return false
		}
		if writes(pass, parent, loop, loop.Value, root, typ) {
			return false
		}
	}
	if _, backward := crossing(parent, loop); backward {
		return false
	}
	if changes(pass, parent, loop, loop.Body.List, mapx) {
		return false
	}
	return !binds(pass, parent, loop, root, typ)
}

// changes reports whether a statement in stmts can change the map.
func changes(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, stmts []ast.Stmt, mapx ast.Expr) bool {
	var (
		root  = base(mapx)
		typ   = pass.TypesInfo.TypeOf(mapx)
		found bool
	)
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				for _, target := range n.Lhs {
					found = found || writes(
						pass, parent, loop,
						target, root, typ,
					)
				}
			case *ast.IncDecStmt:
				found = found || writes(pass, parent, loop, n.X, root, typ)
			case *ast.RangeStmt:
				if n.Tok == token.ASSIGN {
					found = found || writes(
						pass, parent, loop,
						n.Key, root, typ,
					)
					found = found || writes(
						pass, parent, loop,
						n.Value, root, typ,
					)
				}
			case *ast.CallExpr:
				found = found || reaches(pass, parent, loop, n, root, typ)
			case *ast.SendStmt:
				found = found || holds(pass, parent, loop, n.Value, root, typ)
			}
			return !found
		})
		if found {
			break
		}
	}
	return found
}

// writes reports whether an assignment to target changes the map: a write to
// the map, to an element of it, to the value that holds it or a field of it,
// or to another map of its type.
func writes(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, target, root ast.Expr, typ types.Type) bool {
	if target == nil {
		return false
	}
	target = ast.Unparen(target)
	if ident, ok := target.(*ast.Ident); ok {
		if pass.TypesInfo.Defs[ident] != nil {
			return false // := names a new value, so it writes nothing
		}
	}
	if refers(pass, target, root) || alike(pass, target, typ) {
		return true
	}
	index, ok := target.(*ast.IndexExpr)
	if !ok || !alike(pass, index.X, typ) {
		return false
	}
	return !made(pass, outerBody(parent, loop), index.X)
}

// made reports whether e names a map that body builds and never assigns over,
// which nothing outside the function can hold.
func made(pass *analysis.Pass, body *ast.BlockStmt, e ast.Expr) bool {
	ident, ok := ast.Unparen(e).(*ast.Ident)
	if !ok || body == nil {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}
	var built bool
	var binds int
	ast.Inspect(body, func(n ast.Node) bool {
		var targets, values []ast.Expr
		switch n := n.(type) {
		case *ast.AssignStmt:
			targets, values = n.Lhs, n.Rhs
		case *ast.ValueSpec:
			for _, name := range n.Names {
				targets = append(targets, name)
			}
			values = n.Values
		default:
			return true
		}
		for i, target := range targets {
			name, ok := ast.Unparen(target).(*ast.Ident)
			if !ok || pass.TypesInfo.ObjectOf(name) != obj {
				continue
			}
			binds++
			if i < len(values) {
				built = built || builds(pass, values[i])
			}
		}
		return true
	})
	return built && binds == 1
}

// builds reports whether value makes a map rather than naming one.
func builds(pass *analysis.Pass, value ast.Expr) bool {
	switch value := ast.Unparen(value).(type) {
	case *ast.CompositeLit:
		return isMap(pass.TypesInfo.TypeOf(value))
	case *ast.CallExpr:
		ident, ok := ast.Unparen(value.Fun).(*ast.Ident)
		return ok &&
			pass.TypesInfo.Uses[ident] == types.Universe.Lookup("make")
	}
	return false
}

// alike reports whether e has the map's underlying type.
func alike(pass *analysis.Pass, e ast.Expr, typ types.Type) bool {
	t := pass.TypesInfo.TypeOf(e)
	return t != nil && typ != nil && types.Identical(
		t.Underlying(),
		typ.Underlying(),
	)
}

// holds reports whether e can carry the map: its type reaches one, it names a
// value that holds the map and has room for it, or it is an interface
// parameter that the caller can fill.
func holds(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, e, root ast.Expr, typ types.Type) bool {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil || typ == nil {
		return false
	}
	want := typ.Underlying()
	if stores(t, want, false) {
		return true
	}
	if refers(pass, e, root) && stores(t, want, true) {
		return true
	}
	if _, ok := t.Underlying().(*types.Interface); ok {
		return parameter(pass, parent, loop, e)
	}
	return false
}

// parameter reports whether e names a parameter, result, or receiver of a
// function that encloses loop, which nothing else can bind to another map.
func parameter(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, e ast.Expr) bool {
	ident, ok := base(e).(*ast.Ident)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	for n := loop; n != nil; n = parent[n] {
		var lists []*ast.FieldList
		switch n := n.(type) {
		case *ast.FuncDecl:
			lists = []*ast.FieldList{n.Recv, n.Type.Params, n.Type.Results}
		case *ast.FuncLit:
			lists = []*ast.FieldList{n.Type.Params, n.Type.Results}
		}
		for _, list := range lists {
			if list == nil {
				continue
			}
			for _, field := range list.List {
				for _, name := range field.Names {
					if pass.TypesInfo.Defs[name] == obj {
						return true
					}
				}
			}
		}
	}
	return false
}

// stores reports whether a value of type t can store the map: a map of its
// type, or a pointer, container, or struct field that reaches one. An
// interface stores it when named is set, since only a value that names the map
// can have put it there.
func stores(t, want types.Type, named bool) bool {
	return storesIn(t, want, named, make(map[types.Type]bool))
}

// storesIn is stores with a set of types already visited.
func storesIn(t, want types.Type, named bool, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch t := t.Underlying().(type) {
	case *types.Interface:
		return named
	case *types.Map:
		return types.Identical(t, want)
	case *types.Pointer:
		return storesIn(t.Elem(), want, named, seen)
	case *types.Slice:
		return storesIn(t.Elem(), want, named, seen)
	case *types.Array:
		return storesIn(t.Elem(), want, named, seen)
	case *types.Chan:
		return storesIn(t.Elem(), want, named, seen)
	case *types.Struct:
		for field := range t.Fields() {
			if storesIn(field.Type(), want, named, seen) {
				return true
			}
		}
	}
	return false
}

// refers reports whether n names the map or the value that holds it, seeing
// through a call to len or cap, which returns a number.
func refers(pass *analysis.Pass, n ast.Node, root ast.Expr) bool {
	var found bool
	ast.Inspect(n, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.CallExpr:
			if counts(pass, n) {
				return false
			}
		case ast.Expr:
			if astexpr.Same(pass, n, root) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// counts reports whether call is a call to len or cap, which read a container
// and return a number.
func counts(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	switch pass.TypesInfo.Uses[ident] {
	case types.Universe.Lookup("cap"), types.Universe.Lookup("len"):
		return true
	}
	return false
}

// reaches reports whether call can change the map: it hands a value that can
// carry the map to something else, or it calls a function value that may have
// captured the map.
func reaches(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, call *ast.CallExpr, root ast.Expr, typ types.Type) bool {
	if astexpr.Pure(pass, call) {
		return false
	}
	for _, arg := range call.Args {
		if pass.TypesInfo.Types[arg].IsType() {
			continue // make and new name a type rather than a value
		}
		if holds(pass, parent, loop, arg, root, typ) {
			return true
		}
	}
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if holds(pass, parent, loop, sel.X, root, typ) {
			return true
		}
	}
	return functionValue(pass, call.Fun)
}

// functionValue reports whether fun can be a closure that captured the map:
// the callee is neither a conversion, a function declared in a package, nor a
// method.
func functionValue(pass *analysis.Pass, fun ast.Expr) bool {
	switch fun := ast.Unparen(fun).(type) {
	case *ast.Ident:
		switch pass.TypesInfo.Uses[fun].(type) {
		case *types.Builtin, *types.Func, *types.TypeName:
			return false
		}
		return true
	case *ast.SelectorExpr:
		sel, ok := pass.TypesInfo.Selections[fun]
		return ok && sel.Kind() == types.FieldVal
	}
	return true
}

// binds reports whether the function puts the map into a second name, which
// the loop can then write through.
func binds(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, root ast.Expr, typ types.Type) bool {
	body := outerBody(parent, loop)
	if body == nil {
		return false
	}
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			found = pairs(pass, n.Lhs, n.Rhs, root, typ) || found
		case *ast.ValueSpec:
			targets := make([]ast.Expr, len(n.Names))
			for i, name := range n.Names {
				targets[i] = name
			}
			found = pairs(pass, targets, n.Values, root, typ) || found
		}
		return !found
	})
	return found
}

// keeps reports whether assigning value to target carries the map into another
// name: the value names the map or a value that holds it, and the target has
// room for it. A read of one element carries nothing.
func keeps(pass *analysis.Pass, target, value, root ast.Expr, typ types.Type) bool {
	if _, ok := ast.Unparen(value).(*ast.IndexExpr); ok {
		return false
	}
	t := pass.TypesInfo.TypeOf(target)
	return refers(pass, value, root) && stores(t, typ.Underlying(), true)
}

// pairs reports whether assigning one of values to its target carries the map
// into another name.
func pairs(pass *analysis.Pass, targets, values []ast.Expr, root ast.Expr, typ types.Type) bool {
	for i, value := range values {
		if keeps(pass, targets[i], value, root, typ) {
			return true
		}
	}
	return false
}

// within reports whether node lies inside body.
func within(body *ast.BlockStmt, node ast.Node) bool {
	return body.Pos() <= node.Pos() && node.End() <= body.End()
}

// anchorOf returns the node the fix writes before: the loop, or a label on it.
func anchorOf(parent map[ast.Node]ast.Node, loop ast.Node) ast.Node {
	for {
		label, ok := parent[loop].(*ast.LabeledStmt)
		if !ok || label.Stmt != loop {
			return loop
		}
		loop = label
	}
}

// diagnostic returns the report for one call.
func diagnostic(c *call) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:     c.expr.Pos(),
		End:     c.expr.End(),
		Message: "map length does not depend on the loop; evaluate it once before the loop",
	}
}

// decl returns the declaration the fix writes above the loop.
func decl(name string, c *call) string {
	return name + " := len(" + astexpr.Text(c.mapx) + ")"
}

// hoist returns the fix that declares the length above the loop and replaces
// every call in the loop with the name.
func (f *fixer) hoist(cs []*call) analysis.SuggestedFix {
	var (
		first = cs[0]
		edits = []analysis.TextEdit{{
			Pos: first.anchor.Pos(),
			End: first.anchor.Pos(),
			NewText: slices.Concat(
				[]byte(decl(first.name, first)),
				[]byte("\n"),
				indent(f.pass, f.src, first.anchor.Pos()),
			),
		}}
	)
	for _, c := range cs {
		edits = append(edits, analysis.TextEdit{
			Pos:     c.expr.Pos(),
			End:     c.expr.End(),
			NewText: []byte(c.name),
		})
	}
	return analysis.SuggestedFix{
		Message:   "Hoist the length before the loop",
		TextEdits: edits,
	}
}

// fixable reports whether the fix can be written without losing a comment or
// jumping a goto over the loop.
func fixable(pass *analysis.Pass, file *ast.File, src []byte, parent map[ast.Node]ast.Node, c *call) bool {
	forward, _ := crossing(parent, c.anchor)
	return !commented(file, c.expr.Pos(), c.expr.End()) &&
		!attached(pass, file, src, c.anchor.Pos()) &&
		alone(pass, src, c.anchor.Pos()) &&
		!forward
}

// fits reports whether the declaration the fix writes, and the line every call
// in the loop writes the name into, stay within the width limit.
func fits(pass *analysis.Pass, src []byte, cs []*call, name string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	var (
		first = cs[0]
		line  = string(indent(pass, src, first.anchor.Pos())) + decl(
			name,
			first,
		)
	)
	if !linelen.CheckLine([]byte(line)) {
		return false
	}
	for _, c := range cs {
		if !replaced(pass, src, c, name) {
			return false
		}
	}
	return true
}

// indent returns the whitespace that precedes pos on its line, which fixable
// requires to be the whole prefix.
func indent(pass *analysis.Pass, src []byte, pos token.Pos) []byte {
	var (
		tokFile = pass.Fset.File(pos)
		start   = tokFile.LineStart(tokFile.Line(pos))
	)
	return src[tokFile.Offset(start):tokFile.Offset(pos)]
}

// replaced reports whether the line that holds c stays within the limit once
// the name takes the place of the call. A call that spans lines is measured as
// one line, so the answer would be arbitrary, and it reports false.
func replaced(pass *analysis.Pass, src []byte, c *call, name string) bool {
	var (
		tokFile   = pass.Fset.File(c.expr.Pos())
		lineStart = tokFile.LineStart(tokFile.Line(c.expr.Pos()))
		lineEnd   = len(src)
		endLine   = tokFile.Line(c.expr.End())
	)
	if endLine < tokFile.LineCount() {
		lineEnd = tokFile.Offset(tokFile.LineStart(endLine + 1))
	}
	if tokFile.Line(c.expr.Pos()) != tokFile.Line(c.expr.End()) {
		return false
	}
	at := tokFile.Offset(c.expr.Pos())
	end := tokFile.Offset(c.expr.End())
	line := slices.Concat(
		src[tokFile.Offset(lineStart):at],
		[]byte(name),
		bytes.TrimRight(src[end:lineEnd], "\r\n"),
	)
	return linelen.CheckLine(line)
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// attached reports whether a comment of its own ends on pos's line or the line
// above it, where the declaration would separate it from the loop. A comment
// that follows code on its line belongs to that code, and the declaration
// leaves it where it is.
func attached(pass *analysis.Pass, file *ast.File, src []byte, pos token.Pos) bool {
	line := pass.Fset.Position(pos).Line
	for _, group := range file.Comments {
		if group.Pos() >= pos {
			continue
		}
		end := pass.Fset.Position(group.End()).Line
		if end != line && end != line-1 {
			continue
		}
		if alone(pass, src, group.Pos()) {
			return true
		}
	}
	return false
}

// alone reports whether nothing but space precedes pos on its line.
func alone(pass *analysis.Pass, src []byte, pos token.Pos) bool {
	var (
		tokFile = pass.Fset.File(pos)
		start   = tokFile.LineStart(tokFile.Line(pos))
	)
	return source.Blank(src[tokFile.Offset(start):tokFile.Offset(pos)])
}

// functionBody returns the body of the innermost function that holds node.
func functionBody(parent map[ast.Node]ast.Node, node ast.Node) *ast.BlockStmt {
	for n := node; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncDecl:
			return n.Body
		case *ast.FuncLit:
			return n.Body
		}
	}
	return nil
}

// outerBody returns the body of the outermost function that holds node.
func outerBody(parent map[ast.Node]ast.Node, node ast.Node) (body *ast.BlockStmt) {
	for n := node; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncDecl:
			return n.Body
		case *ast.FuncLit:
			body = n.Body
		}
	}
	return body
}

// crossing reports whether a goto in the function crosses the anchor: one from
// before it passes a declaration Go rejects, and one from after it re-enters
// the loop without running it.
func crossing(parent map[ast.Node]ast.Node, anchor ast.Node) (forward, backward bool) {
	body := functionBody(parent, anchor)
	if body == nil {
		return false, false
	}
	labels := make(map[string]token.Pos)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n.Label.Pos()
		}
		return true
	})
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if n.Tok != token.GOTO {
				return true
			}
			target, ok := labels[n.Label.Name]
			if !ok {
				return true
			}
			if n.Pos() < anchor.Pos() && target >= anchor.Pos() {
				forward = true
			}
			if n.Pos() > anchor.End() && target <= anchor.Pos() {
				backward = true
			}
		}
		return !forward || !backward
	})
	return forward, backward
}

// span returns the positions just inside the statement list that owner holds,
// and at its end.
func span(owner ast.Node) (start, end token.Pos) {
	switch owner := owner.(type) {
	case *ast.BlockStmt:
		return owner.Lbrace, owner.Rbrace + 1
	case *ast.CaseClause:
		return owner.Colon, owner.End()
	case *ast.CommClause:
		return owner.Colon, owner.End()
	}
	return token.NoPos, token.NoPos
}

// declareScope returns the scope a declaration at the top of block lands in,
// or nil when the package has no scope there.
func declareScope(pass *analysis.Pass, block ast.Node) *types.Scope {
	start, _ := span(block)
	return pass.Pkg.Scope().Innermost(start)
}
