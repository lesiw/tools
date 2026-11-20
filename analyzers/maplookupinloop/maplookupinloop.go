// Package maplookupinloop provides an analysis.Analyzer that reports a map
// lookup in a loop whose map and key do not change.
//
// The section of the spec on index expressions says a map index yields the
// element for a key (https://go.dev/ref/spec#Index_expressions). A lookup that
// reads the same map with the same key returns the same element on every pass,
// so one lookup before the loop can replace the one in the body.
//
// The fix hoists the lookup above the loop and replaces the index expression
// with the value or values it bound. It is withheld where a comment is inside
// the lookup or immediately before the loop, where a name would leave scope,
// where a goto would jump over the declaration, and where evaluating the key
// could panic; the README lists the cases.
//
// The rule is adapted from staticcheck's SA6001.
package maplookupinloop

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "maplookupinloop",
	Doc:  "reports a map lookup in a loop whose operands do not change",
	Run:  run,
}

// lookup is one map index expression this rule reports.
type lookup struct {
	index  *ast.IndexExpr
	loop   ast.Node
	mapx   ast.Expr
	key    ast.Expr
	two    bool // the index supplies both the value and the presence flag
	direct bool // the index is not parenthesized in a two-result form
	fix    bool
	decl   bool // this lookup writes the declaration for its value
	anchor ast.Node
	name   string
	okName string
}

// run finds the lookups in each file, groups the fixable ones by loop, and
// reports them.
func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if err := checkFile(pass, file); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// checkFile reports the lookups in one file, with a fix for each loop whose
// declarations fit the width limit.
func checkFile(pass *analysis.Pass, file *ast.File) error {
	name := pass.Fset.File(file.Pos()).Name()
	src, err := pass.ReadFile(name)
	if err != nil {
		return err
	}
	parent := astexpr.Parents(file)
	used := names(pass, file)
	found := lookups(pass, parent, file)
	order, groups := fixableGroups(pass, file, parent, found)
	fixes := make(map[ast.Node]analysis.SuggestedFix, len(groups))
	for _, loop := range order {
		if fix, ok := loopFix(pass, src, used, groups[loop]); ok {
			fixes[loop] = fix
		}
	}
	for _, l := range found {
		d := diagnostic(l)
		if l.fix {
			d.SuggestedFixes = []analysis.SuggestedFix{fixes[l.loop]}
		}
		pass.Report(d)
	}
	return nil
}

// lookups returns the reported lookups in file.
func lookups(pass *analysis.Pass, parent map[ast.Node]ast.Node, file *ast.File) []*lookup {
	var found []*lookup
	ast.Inspect(file, func(n ast.Node) bool {
		index, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		l, ok := lookupOf(pass, parent, index)
		if ok {
			found = append(found, l)
		}
		return true
	})
	return found
}

// fixableGroups returns the fixable lookups grouped by loop, in source order.
func fixableGroups(pass *analysis.Pass, file *ast.File, parent map[ast.Node]ast.Node, found []*lookup) ([]ast.Node, map[ast.Node][]*lookup) {
	var (
		order  []ast.Node
		groups = make(map[ast.Node][]*lookup)
	)
	for _, l := range found {
		if !fixable(pass, file, parent, l) {
			continue
		}
		l.fix = true
		if _, ok := groups[l.loop]; !ok {
			order = append(order, l.loop)
		}
		groups[l.loop] = append(groups[l.loop], l)
	}
	return order, groups
}

// loopFix names the lookups in one loop and returns the fix for it.
func loopFix(pass *analysis.Pass, src []byte, used map[string]bool, ls []*lookup) (analysis.SuggestedFix, bool) {
	var primary []*lookup
	for _, l := range ls {
		if prev := sameAs(pass, primary, l); prev != nil {
			l.name, l.okName = prev.name, prev.okName
			continue
		}
		l.decl = true
		l.name = pick(used, "lookup")
		if l.two {
			l.okName = pick(used, "found")
		}
		primary = append(primary, l)
	}
	blocked := make(map[string]struct{})
	for _, l := range ls {
		if l.decl && !fits(pass, src, l) {
			blocked[l.name] = struct{}{}
		}
	}
	var kept []*lookup
	for _, l := range ls {
		if _, ok := blocked[l.name]; ok {
			l.fix = false
			continue
		}
		if !fits(pass, src, l) {
			l.fix = false
			continue
		}
		kept = append(kept, l)
	}
	if len(kept) == 0 {
		return analysis.SuggestedFix{}, false
	}
	return hoist(kept), true
}

// lookupOf returns index as a reported lookup, and reports false for an index
// that this rule leaves alone.
func lookupOf(pass *analysis.Pass, parent map[ast.Node]ast.Node, index *ast.IndexExpr) (*lookup, bool) {
	if pass.TypesInfo.Types[index].IsType() {
		return nil, false
	}
	if directTarget(parent, index) || benchmarkLoop(parent, index) {
		return nil, false
	}
	keyType, ok := mapOf(pass, index.X)
	if !ok || !astexpr.NoInterface(keyType.Key()) {
		return nil, false
	}
	loop, ok := enclosingLoop(parent, index)
	if !ok {
		return nil, false
	}
	two, direct := resultCount(parent, index)
	if !invariant(pass, parent, loop, index.X, index.Index) {
		return nil, false
	}
	return &lookup{
		index:  index,
		loop:   loop,
		mapx:   index.X,
		key:    index.Index,
		two:    two,
		direct: direct,
	}, true
}

// directTarget reports whether index is itself the target of an assignment or
// an increment. Such an index writes the map, and the rule reports reads.
func directTarget(parent map[ast.Node]ast.Node, index ast.Node) bool {
	for node := index; ; {
		next := parent[node]
		switch next := next.(type) {
		case *ast.ParenExpr:
			node = next
			continue
		case *ast.AssignStmt:
			for _, target := range next.Lhs {
				if target == node {
					return true
				}
			}
		case *ast.IncDecStmt:
			return next.X == node
		case *ast.RangeStmt:
			return next.Key == node || next.Value == node
		}
		return false
	}
}

// benchmarkLoop reports whether node is in a function whose name contains
// benchmark. A benchmark repeats a lookup on purpose, so hoisting would
// remove the work it measures.
func benchmarkLoop(parent map[ast.Node]ast.Node, node ast.Node) bool {
	for n := parent[node]; n != nil; n = parent[n] {
		if decl, ok := n.(*ast.FuncDecl); ok {
			return strings.Contains(
				strings.ToLower(decl.Name.Name),
				"benchmark",
			)
		}
	}
	return false
}

// resultCount reports whether the index supplies two values, and whether it is
// directly the right-hand side of that form. A two-result form is an
// assignment or a var declaration with two names on the left and one value on
// the right.
func resultCount(parent map[ast.Node]ast.Node, index *ast.IndexExpr) (two, direct bool) {
	var node ast.Node = index
	for {
		var (
			next      = parent[node]
			paren, ok = next.(*ast.ParenExpr)
		)
		if !ok {
			switch next := next.(type) {
			case *ast.AssignStmt:
				if len(next.Lhs) == 2 && len(next.Rhs) == 1 {
					return true, next.Rhs[0] == index
				}
			case *ast.ValueSpec:
				if len(next.Names) == 2 && len(next.Values) == 1 {
					return true, next.Values[0] == index
				}
			}
			return false, true
		}
		node = paren
	}
}

// enclosingLoop returns the loop whose body holds node. It reports false for a
// node outside a loop body, and for a node in a function literal with no loop
// inside it.
func enclosingLoop(parent map[ast.Node]ast.Node, node ast.Node) (ast.Node, bool) {
	for n := parent[node]; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncLit:
			return nil, false
		case *ast.ForStmt:
			if within(n.Body, node) {
				return n, true
			}
		case *ast.RangeStmt:
			if within(n.Body, node) {
				return n, true
			}
		}
	}
	return nil, false
}

// within reports whether node lies inside body.
func within(body *ast.BlockStmt, node ast.Node) bool {
	return body.Pos() <= node.Pos() && node.End() <= body.End()
}

// invariant reports whether loop reads mapx with key on every pass and does
// not change either one.
func invariant(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, mapx, key ast.Expr) bool {
	var (
		mapTaint = taintOf(pass, parent, loop, mapx)
		keyTaint = taintOf(pass, parent, loop, key)
	)
	if packageMap(pass, mapx) || packageValue(pass, key) {
		return false
	}
	if packageTaint(mapTaint) || packageTaint(keyTaint) {
		return false
	}
	if otherMapValue(pass, parent, loop, mapx) {
		return false
	}
	if mapFromCall(pass, parent, loop, mapx) {
		return false
	}
	if keyPointer(pass, parent, loop, key) {
		return false
	}
	if escapes(pass, parent, loop, mapTaint) {
		return false
	}
	if escapes(pass, parent, loop, keyTaint) {
		return false
	}
	if !stableMap(pass, mapx) || !stableKey(pass, key) {
		return false
	}
	writes := writesIn(pass, loop)
	if !sameMap(pass, mapx, mapTaint, writes) {
		return false
	}
	return !changed(pass, key, keyTaint, writes)
}

// taintOf returns the objects that can hold the value of expr, following
// assignments, aliases, containers, range variables, and type switches in the
// function that holds loop.
func taintOf(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, expr ast.Expr) map[types.Object]bool {
	var (
		taint = objectsOf(pass, expr)
		body  = outerBody(parent, loop)
	)
	if body == nil {
		return taint
	}
	for {
		var found bool
		ast.Inspect(body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				found = taintList(pass, n.Lhs, n.Rhs, taint) || found
			case *ast.ValueSpec:
				found = taintValues(pass, n.Names, n.Values, taint) || found
			case *ast.RangeStmt:
				if n.X != nil && mentionsObject(pass, n.X, taint) {
					found = taintTarget(pass, n.Key, taint) || found
					found = taintTarget(pass, n.Value, taint) || found
				}
			case *ast.TypeSwitchStmt:
				for _, stmt := range n.Body.List {
					clause, ok := stmt.(*ast.CaseClause)
					if !ok {
						continue
					}
					found = taintObject(
						pass.TypesInfo.Implicits[clause],
						taint,
					) || found
				}
			}
			return true
		})
		if !found {
			return taint
		}
	}
}

// taintList follows one assignment in both directions.
func taintList(pass *analysis.Pass, targets, values []ast.Expr, taint map[types.Object]bool) (found bool) {
	for i, value := range values {
		if i >= len(targets) {
			break
		}
		found = taintPair(pass, targets[i], value, taint) || found
	}
	return found
}

// taintValues follows one var declaration in both directions.
func taintValues(pass *analysis.Pass, names []*ast.Ident, values []ast.Expr, taint map[types.Object]bool) (found bool) {
	for i, value := range values {
		if i >= len(names) {
			break
		}
		found = taintPair(pass, names[i], value, taint) || found
	}
	return found
}

// taintPair adds the target when the value can hold a tainted object, and adds
// the value's objects when the target is already tainted.
func taintPair(pass *analysis.Pass, target, value ast.Expr, taint map[types.Object]bool) (found bool) {
	if aliasExpr(pass, value, taint) {
		found = taintTarget(pass, target, taint) || found
	}
	obj := rootObject(pass, target)
	if obj != nil && taint[obj] {
		found = taintObjects(pass, value, taint) || found
	}
	return found
}

// aliasExpr reports whether value can hold the same object as one in taint. A
// name, field, pointer, composite literal, or non-basic result can; a scalar
// computed from the object cannot.
func aliasExpr(pass *analysis.Pass, value ast.Expr, taint map[types.Object]bool) bool {
	if !mentionsObject(pass, value, taint) {
		return false
	}
	switch value := ast.Unparen(value).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.StarExpr, *ast.CompositeLit,
		*ast.FuncLit:
		return true
	case *ast.CallExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.TypeAssertExpr,
		*ast.UnaryExpr:
		return !basic(pass.TypesInfo.TypeOf(value))
	}
	return false
}

// basic reports whether t holds only scalar values, as a map index's value
// does when it is used in an assignment.
func basic(t types.Type) bool {
	if t == nil {
		return false
	}
	if tuple, ok := t.(*types.Tuple); ok {
		for v := range tuple.Variables() {
			if !basic(v.Type()) {
				return false
			}
		}
		return tuple.Len() > 0
	}
	_, ok := t.Underlying().(*types.Basic)
	return ok
}

// taintTarget adds the object a target names to the taint set.
func taintTarget(pass *analysis.Pass, target ast.Expr, taint map[types.Object]bool) bool {
	if target == nil {
		return false
	}
	return taintObject(rootObject(pass, target), taint)
}

// taintObjects adds every object in expr to the taint set.
func taintObjects(pass *analysis.Pass, expr ast.Expr, taint map[types.Object]bool) (found bool) {
	for obj := range objectsOf(pass, expr) {
		found = taintObject(obj, taint) || found
	}
	return found
}

// taintObject adds obj to the taint set.
func taintObject(obj types.Object, taint map[types.Object]bool) bool {
	if _, ok := obj.(*types.Var); !ok || taint[obj] {
		return false
	}
	taint[obj] = true
	return true
}

// outerBody returns the outermost function body that holds node.
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

// escapes reports whether a pointer, a function literal, or a call outside a
// pure builtin can let another value change one in taint while loop runs.
func escapes(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, taint map[types.Object]bool) bool {
	body := functionBody(parent, loop)
	if body == nil {
		return false
	}
	if scanEscapes(pass, body, taint, nil) {
		return true
	}
	outer := outerBody(parent, loop)
	if outer == body {
		return false
	}
	return scanEscapes(pass, outer, taint, nearestLiteral(parent, loop))
}

// scanEscapes reports whether body calls a value that can change taint.
func scanEscapes(pass *analysis.Pass, body ast.Node, taint map[types.Object]bool, skip ast.Node) bool {
	var escaped bool
	ast.Inspect(body, func(n ast.Node) bool {
		if n == skip {
			return false
		}
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				escaped = true
			}
			if n.Op == token.AND && mentionsObject(pass, n.X, taint) {
				escaped = true
			}
		case *ast.SendStmt:
			escaped = true
		case *ast.RangeStmt:
			typ := pass.TypesInfo.TypeOf(n.X)
			if typeHas(typ, isSignature) || typeHas(typ, isChan) {
				escaped = true
			}
		case *ast.CallExpr:
			literal, ok := ast.Unparen(n.Fun).(*ast.FuncLit)
			if ok && literal == skip {
				return false
			}
			if !harmless(pass, n) {
				escaped = true
			}
		}
		return !escaped
	})
	return escaped
}

// nearestLiteral returns the function literal that holds loop, or nil.
func nearestLiteral(parent map[ast.Node]ast.Node, loop ast.Node) ast.Node {
	for n := loop; n != nil; n = parent[n] {
		if literal, ok := n.(*ast.FuncLit); ok {
			return literal
		}
	}
	return nil
}

// harmless reports whether call is a conversion or a builtin that cannot reach
// an existing map or key.
func harmless(pass *analysis.Pass, call *ast.CallExpr) bool {
	if pass.TypesInfo.Types[call.Fun].IsType() {
		return true
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	if !ok {
		return false
	}
	switch builtin.Name() {
	case "cap", "complex", "imag", "len", "make", "max", "min", "new", "real":
		return true
	}
	return false
}

// packageValue reports whether key names a package-level variable, which a
// call can change without naming it.
func packageValue(pass *analysis.Pass, key ast.Expr) bool {
	var found bool
	ast.Inspect(key, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if packageVar(pass.TypesInfo.Uses[n]) {
				found = true
			}
		case *ast.SelectorExpr:
			if packageVar(pass.TypesInfo.Uses[n.Sel]) {
				found = true
			}
		}
		return !found
	})
	return found
}

// packageVar reports whether obj is a variable declared at package level.
func packageVar(obj types.Object) bool {
	variable, ok := obj.(*types.Var)
	return ok && variable.Parent() == variable.Pkg().Scope()
}

// signatureObjects returns the named parameters, results, and receivers of
// the functions and literals that hold loop.
func signatureObjects(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node) (found []types.Object) {
	for n := loop; n != nil; n = parent[n] {
		switch n := n.(type) {
		case *ast.FuncDecl:
			found = append(found, fieldObjects(pass, n.Recv)...)
			found = append(found, fieldObjects(pass, n.Type.Params)...)
			found = append(found, fieldObjects(pass, n.Type.Results)...)
		case *ast.FuncLit:
			found = append(found, fieldObjects(pass, n.Type.Params)...)
			found = append(found, fieldObjects(pass, n.Type.Results)...)
		}
	}
	return found
}

// fieldObjects returns the objects that a field list declares.
func fieldObjects(pass *analysis.Pass, fields *ast.FieldList) (found []types.Object) {
	if fields == nil {
		return nil
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if obj := pass.TypesInfo.Defs[name]; obj != nil {
				found = append(found, obj)
			}
		}
	}
	return found
}

// keyPointer reports whether the function holds a pointer to the key's type,
// which can change the key through the caller.
func keyPointer(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, key ast.Expr) bool {
	var (
		body    = outerBody(parent, loop)
		keyType = pass.TypesInfo.TypeOf(key)
	)
	if body == nil || keyType == nil {
		return false
	}
	match := func(t types.Type) bool {
		pointer, ok := t.Underlying().(*types.Pointer)
		return ok && types.Identical(pointer.Elem(), keyType)
	}
	for _, obj := range signatureObjects(pass, parent, loop) {
		if typeHas(obj.Type(), match) {
			return true
		}
	}
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		var obj types.Object
		switch n := n.(type) {
		case *ast.Ident:
			obj = pass.TypesInfo.Defs[n]
			if obj == nil {
				obj = pass.TypesInfo.Uses[n]
			}
		case *ast.SelectorExpr:
			obj = pass.TypesInfo.Uses[n.Sel]
		}
		if _, ok := obj.(*types.Var); ok && typeHas(obj.Type(), match) {
			found = true
		}
		return !found
	})
	return found
}

// mapFromCall reports whether the function binds mapx from a call, which can
// return a package-level map that another call changes. make is excluded,
// since it returns a fresh map.
func mapFromCall(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, mapx ast.Expr) bool {
	body := outerBody(parent, loop)
	if body == nil {
		return false
	}
	root := rootObject(pass, mapx)
	var found bool
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
			if _, ok := ast.Unparen(target).(*ast.IndexExpr); ok {
				continue
			}
			if rootObject(pass, target) != root {
				continue
			}
			value := pairedValue(values, i, len(targets))
			if value != nil && unsafeInit(pass, value) {
				found = true
			}
		}
		return !found
	})
	return found
}

// pairedValue returns the value assigned to target i of count targets. One
// value can supply every target of a multi-value call.
func pairedValue(values []ast.Expr, i, count int) ast.Expr {
	if len(values) == 1 && count > 1 {
		return values[0]
	}
	if i < len(values) {
		return values[i]
	}
	return nil
}

// unsafeInit reports whether value can return a map from a call rather than
// build a fresh one.
func unsafeInit(pass *analysis.Pass, value ast.Expr) bool {
	switch value := ast.Unparen(value).(type) {
	case *ast.CallExpr:
		return !makeCall(pass, value)
	case *ast.Ident, *ast.CompositeLit:
		return false
	case *ast.SelectorExpr:
		return !simpleName(value.X)
	}
	return true
}

// simpleName reports whether e is a name or a field path.
func simpleName(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return simpleName(e.X)
	}
	return false
}

// makeCall reports whether call is the make builtin.
func makeCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	builtin, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[builtin] == types.Universe.Lookup("make")
}

// otherMapValue reports whether the function names a second value that can
// hold a map, which the caller can pass as the same map as mapx.
func otherMapValue(pass *analysis.Pass, parent map[ast.Node]ast.Node, loop ast.Node, mapx ast.Expr) bool {
	body := outerBody(parent, loop)
	if body == nil {
		return false
	}
	target, root := mapObject(pass, mapx), rootObject(pass, mapx)
	for _, obj := range signatureObjects(pass, parent, loop) {
		if obj == target || obj == root {
			continue
		}
		if _, ok := obj.(*types.Var); ok && typeHas(obj.Type(), canHoldMap) {
			return true
		}
	}
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		var obj types.Object
		switch n := n.(type) {
		case *ast.Ident:
			obj = pass.TypesInfo.Defs[n]
			if obj == nil {
				obj = pass.TypesInfo.Uses[n]
			}
		case *ast.SelectorExpr:
			obj = pass.TypesInfo.Uses[n.Sel]
		}
		if _, ok := obj.(*types.Var); !ok || obj == target || obj == root {
			return true
		}
		if typeHas(obj.Type(), canHoldMap) {
			found = true
		}
		return !found
	})
	return found
}

// mapObject returns the object that mapx names.
func mapObject(pass *analysis.Pass, mapx ast.Expr) types.Object {
	switch mapx := ast.Unparen(mapx).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[mapx]
	case *ast.SelectorExpr:
		return pass.TypesInfo.Uses[mapx.Sel]
	}
	return nil
}

// typeHas reports whether t or a type it holds satisfies match.
func typeHas(t types.Type, match func(types.Type) bool) bool {
	return typeHasSeen(t, match, make(map[types.Type]bool))
}

// typeHasSeen is typeHas with a set of types already visited.
func typeHasSeen(t types.Type, match func(types.Type) bool, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	if match(t) {
		return true
	}
	if param, ok := t.(*types.TypeParam); ok {
		return typeHasSeen(param.Constraint(), match, seen)
	}
	switch t := t.Underlying().(type) {
	case *types.Pointer:
		return typeHasSeen(t.Elem(), match, seen)
	case *types.Slice:
		return typeHasSeen(t.Elem(), match, seen)
	case *types.Array:
		return typeHasSeen(t.Elem(), match, seen)
	case *types.Chan:
		return typeHasSeen(t.Elem(), match, seen)
	case *types.Struct:
		for field := range t.Fields() {
			if typeHasSeen(field.Type(), match, seen) {
				return true
			}
		}
	case *types.Interface:
		for embedded := range t.EmbeddedTypes() {
			if typeHasSeen(embedded, match, seen) {
				return true
			}
		}
	case *types.Union:
		for term := range t.Terms() {
			if typeHasSeen(term.Type(), match, seen) {
				return true
			}
		}
	}
	return false
}

// canHoldMap reports whether t can hold a map value.
func canHoldMap(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Map, *types.Interface, *types.Signature:
		return true
	}
	return false
}

// isSlice reports whether t is a slice type.
func isSlice(t types.Type) bool {
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// isSignature reports whether t is a function type.
func isSignature(t types.Type) bool {
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

// isChan reports whether t is a channel type.
func isChan(t types.Type) bool {
	_, ok := t.Underlying().(*types.Chan)
	return ok
}

// packageTaint reports whether taint holds a package-level value, which a call
// can change without naming it.
func packageTaint(taint map[types.Object]bool) bool {
	for obj := range taint {
		if packageVar(obj) {
			return true
		}
	}
	return false
}

// packageMap reports whether mapx is a package-level value, which a call the
// loop makes can change without naming it.
func packageMap(pass *analysis.Pass, mapx ast.Expr) bool {
	obj := rootObject(pass, mapx)
	if _, ok := obj.(*types.PkgName); ok {
		return true
	}
	variable, ok := obj.(*types.Var)
	return ok && variable.Parent() == pass.Pkg.Scope()
}

// objectsOf returns the objects that expr names.
func objectsOf(pass *analysis.Pass, expr ast.Expr) map[types.Object]bool {
	found := make(map[types.Object]bool)
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if obj := pass.TypesInfo.Uses[n]; obj != nil {
				found[obj] = true
			}
		case *ast.SelectorExpr:
			if obj := pass.TypesInfo.Uses[n.Sel]; obj != nil {
				found[obj] = true
			}
		}
		return true
	})
	return found
}

// mentionsObject reports whether n names one of objects.
func mentionsObject(pass *analysis.Pass, n ast.Node, objects map[types.Object]bool) bool {
	var found bool
	ast.Inspect(n, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok && objects[pass.TypesInfo.Uses[ident]] {
			found = true
		}
		return !found
	})
	return found
}

// writes holds the assignments a loop can make.
type writes struct {
	objects map[types.Object]bool
	targets []ast.Expr
}

// writesIn returns the assignments in loop, including the loop header and any
// nested function literal.
func writesIn(pass *analysis.Pass, loop ast.Node) *writes {
	w := &writes{objects: make(map[types.Object]bool)}
	ast.Inspect(loop, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, target := range n.Lhs {
				w.add(pass, target)
			}
		case *ast.IncDecStmt:
			w.add(pass, n.X)
		case *ast.CallExpr:
			sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr)
			if !ok {
				break
			}
			if _, ok := pass.TypesInfo.Selections[sel]; !ok {
				break
			}
			if obj := rootObject(pass, sel.X); obj != nil {
				w.objects[obj] = true
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				w.add(pass, n.X)
			}
		case *ast.RangeStmt:
			w.add(pass, n.Key)
			w.add(pass, n.Value)
		case *ast.TypeSwitchStmt:
			for _, stmt := range n.Body.List {
				clause, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				if obj := pass.TypesInfo.Implicits[clause]; obj != nil {
					w.objects[obj] = true
				}
			}
		case *ast.ValueSpec:
			for _, name := range n.Names {
				if obj, ok := pass.TypesInfo.Defs[name].(*types.Var); ok {
					w.objects[obj] = true
					w.targets = append(w.targets, name)
				}
			}
		}
		return true
	})
	return w
}

// add records one assignment target.
func (w *writes) add(pass *analysis.Pass, target ast.Expr) {
	if target == nil {
		return
	}
	target = ast.Unparen(target)
	if ident, ok := target.(*ast.Ident); ok && ident.Name == "_" {
		return
	}
	w.targets = append(w.targets, target)
	if obj := targetObject(pass, target); obj != nil {
		w.objects[obj] = true
	}
	if obj := rootObject(pass, target); obj != nil {
		w.objects[obj] = true
	}
}

// targetObject returns the object a target assigns. A selector target names
// its field, and an index or a dereference target names no variable.
func targetObject(pass *analysis.Pass, target ast.Expr) types.Object {
	switch target := ast.Unparen(target).(type) {
	case *ast.Ident:
		if obj := pass.TypesInfo.Defs[target]; obj != nil {
			return obj
		}
		return pass.TypesInfo.Uses[target]
	case *ast.SelectorExpr:
		return pass.TypesInfo.Uses[target.Sel]
	}
	return nil
}

// otherMapWrite reports whether the loop writes a map other than mapx, which
// the caller can pass as the same map.
func otherMapWrite(pass *analysis.Pass, mapx ast.Expr, w *writes) bool {
	for _, target := range w.targets {
		index, ok := ast.Unparen(target).(*ast.IndexExpr)
		if !ok {
			continue
		}
		if _, ok := mapOf(pass, index.X); !ok {
			if astexpr.Mentions(pass, index.X, mapx) {
				return true
			}
			continue
		}
		if !astexpr.Same(pass, index.X, mapx) {
			return true
		}
	}
	return false
}

// changed reports whether a write in the loop can change key. A write to a
// value key names, or to an object that can hold the same value, counts.
func changed(pass *analysis.Pass, key ast.Expr, taint map[types.Object]bool, w *writes) bool {
	if obj := rootObject(pass, key); obj != nil && w.objects[obj] {
		return true
	}
	for _, target := range w.targets {
		if encloses(pass, key, target) || astexpr.Mentions(pass, key, target) {
			return true
		}
		if obj := rootObject(pass, target); obj != nil && taint[obj] {
			return true
		}
	}
	return false
}

// encloses reports whether outer is inner or a prefix of inner, as a name is a
// prefix of a field path or an index expression.
func encloses(pass *analysis.Pass, outer, inner ast.Expr) bool {
	if astexpr.Same(pass, outer, inner) {
		return true
	}
	switch inner := ast.Unparen(inner).(type) {
	case *ast.SelectorExpr:
		return encloses(pass, outer, inner.X)
	case *ast.IndexExpr:
		return encloses(pass, outer, inner.X)
	case *ast.SliceExpr:
		return encloses(pass, outer, inner.X)
	case *ast.StarExpr:
		return encloses(pass, outer, inner.X)
	}
	return false
}

// rootObject returns the object that the leftmost name in e refers to.
func rootObject(pass *analysis.Pass, e ast.Expr) types.Object {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if obj := pass.TypesInfo.Defs[e]; obj != nil {
			return obj
		}
		return pass.TypesInfo.Uses[e]
	case *ast.SelectorExpr:
		return rootObject(pass, e.X)
	case *ast.IndexExpr:
		return rootObject(pass, e.X)
	case *ast.SliceExpr:
		return rootObject(pass, e.X)
	case *ast.StarExpr:
		return rootObject(pass, e.X)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return rootObject(pass, e.X)
		}
	case *ast.TypeAssertExpr:
		return rootObject(pass, e.X)
	}
	return nil
}

// stableMap reports whether mapx names a map that the loop can read the same
// way on every pass: a name or a field path.
func stableMap(pass *analysis.Pass, mapx ast.Expr) bool {
	switch mapx := ast.Unparen(mapx).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[mapx] != nil
	case *ast.SelectorExpr:
		if _, ok := pass.TypesInfo.Selections[mapx]; ok {
			if !astexpr.Follows(pass, mapx) {
				return false
			}
		}
		return stableMap(pass, mapx.X)
	}
	return false
}

// stableKey reports whether key has a form whose value cannot change from one
// pass to the next. A name is checked against the loop's assignments, and a
// call is allowed only when it is a conversion or a builtin that only reads
// its arguments.
func stableKey(pass *analysis.Pass, key ast.Expr) bool {
	switch key := ast.Unparen(key).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[key] != nil
	case *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		return stableSelector(pass, key)
	case *ast.UnaryExpr:
		switch key.Op {
		case token.ADD, token.NOT, token.SUB, token.XOR:
			return stableKey(pass, key.X)
		}
	case *ast.BinaryExpr:
		return stableKey(pass, key.X) && stableKey(pass, key.Y)
	case *ast.CallExpr:
		return stableCall(pass, key)
	case *ast.CompositeLit:
		return stableLiteral(pass, key)
	}
	return false
}

// stableSelector reports whether key reads a field of a value that does not
// go through a pointer.
func stableSelector(pass *analysis.Pass, key *ast.SelectorExpr) bool {
	if sel, ok := pass.TypesInfo.Selections[key]; ok {
		if sel.Kind() != types.FieldVal || sel.Indirect() {
			return false
		}
		if pass.TypesInfo.Uses[key.Sel] == nil {
			return false
		}
	}
	return stableKey(pass, key.X)
}

// stableCall reports whether key is a conversion or a builtin that only reads
// its arguments.
func stableCall(pass *analysis.Pass, key *ast.CallExpr) bool {
	if pass.TypesInfo.Types[key.Fun].IsType() {
		if len(key.Args) == 1 {
			from := pass.TypesInfo.TypeOf(key.Args[0])
			if typeHas(from, isSlice) {
				return false
			}
		}
		return allStable(pass, key.Args)
	}
	id, ok := ast.Unparen(key.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	if !ok {
		return false
	}
	switch builtin.Name() {
	case "cap", "len":
		for _, arg := range key.Args {
			if typ := pass.TypesInfo.TypeOf(arg); typeHas(typ, isChan) {
				return false
			}
		}
		return allStable(pass, key.Args)
	case "max", "min":
		return allStable(pass, key.Args)
	}
	return false
}

// stableLiteral reports whether every element of key is stable.
func stableLiteral(pass *analysis.Pass, key *ast.CompositeLit) bool {
	for _, elt := range key.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if !stableKey(pass, kv.Key) || !stableKey(pass, kv.Value) {
				return false
			}
			continue
		}
		expr, ok := elt.(ast.Expr)
		if !ok || !stableKey(pass, expr) {
			return false
		}
	}
	return true
}

// allStable reports whether every expression in exprs is stable.
func allStable(pass *analysis.Pass, exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if !stableKey(pass, expr) {
			return false
		}
	}
	return true
}

// sameMap reports whether loop leaves mapx alone: no assignment replaces it,
// and no indexed assignment writes through it.
func sameMap(pass *analysis.Pass, mapx ast.Expr, taint map[types.Object]bool, w *writes) bool {
	if obj := rootObject(pass, mapx); obj != nil && w.objects[obj] {
		return false
	}
	if otherMapWrite(pass, mapx, w) {
		return false
	}
	for e := mapx; ; {
		for _, target := range w.targets {
			if astexpr.Same(pass, target, e) {
				return false
			}
			index, ok := ast.Unparen(target).(*ast.IndexExpr)
			if ok && (index.X == e || astexpr.Same(pass, index.X, e)) {
				return false
			}
			if obj := rootObject(pass, target); obj != nil && taint[obj] {
				return false
			}
		}
		sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
		if !ok {
			return true
		}
		e = sel.X
	}
}

// fixable reports whether the lookup can be hoisted without changing what the
// program does or losing a comment.
func fixable(pass *analysis.Pass, file *ast.File, parent map[ast.Node]ast.Node, l *lookup) bool {
	if !l.direct || commented(file, l.index.Pos(), l.index.End()) {
		return false
	}
	if !astexpr.Safe(pass, l.mapx) || !astexpr.Safe(pass, l.key) {
		return false
	}
	typ := pass.TypesInfo.TypeOf(l.key)
	if typ == nil || !astexpr.NoInterface(typ) {
		return false
	}
	l.anchor = anchorOf(parent, l.loop)
	if attached(pass, file, l.anchor.Pos()) || jumpedOver(parent, l.anchor) {
		return false
	}
	return inScope(pass, l.mapx, l.anchor.Pos()) &&
		inScope(pass, l.key, l.anchor.Pos())
}

// sameAs returns the lookup in primary that reads the same map and key with
// the same number of results, or nil when none does. The first of a repeated
// lookup writes the declaration that the rest share.
func sameAs(pass *analysis.Pass, primary []*lookup, l *lookup) *lookup {
	for _, prev := range primary {
		if prev.two != l.two {
			continue
		}
		if sameExpr(pass, prev.mapx, l.mapx) {
			if sameExpr(pass, prev.key, l.key) {
				return prev
			}
		}
	}
	return nil
}

// sameExpr reports whether a and b print the same and name the same objects.
func sameExpr(pass *analysis.Pass, a, b ast.Expr) bool {
	if astexpr.Text(a) != astexpr.Text(b) {
		return false
	}
	left, right := objects(pass, a), objects(pass, b)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// objects returns the objects that the identifiers in e name, in source order.
func objects(pass *analysis.Pass, e ast.Expr) []types.Object {
	var found []types.Object
	ast.Inspect(e, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			found = append(found, pass.TypesInfo.ObjectOf(ident))
		}
		return true
	})
	return found
}

// fits reports whether the lines the fix writes stay within the width limit.
func fits(pass *analysis.Pass, src []byte, l *lookup) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	var (
		tokFile = pass.Fset.File(l.anchor.Pos())
		start   = tokFile.LineStart(tokFile.Line(l.anchor.Pos()))
		indent  = src[tokFile.Offset(start):tokFile.Offset(l.anchor.Pos())]
		decl    = l.name
	)
	if l.two {
		decl += ", " + l.okName
	}
	decl += " := " + astexpr.Text(l.mapx) + "[" + astexpr.Text(l.key) + "]"
	block := []byte(string(indent) + decl)
	for line := range bytes.SplitSeq(block, []byte("\n")) {
		if !linelen.CheckLine(line) {
			return false
		}
	}
	lineStart := tokFile.LineStart(tokFile.Line(l.index.Pos()))
	lineEnd := len(src)
	endLine := tokFile.Line(l.index.End())
	if endLine < tokFile.LineCount() {
		lineEnd = tokFile.Offset(tokFile.LineStart(endLine + 1))
	}
	prefix := src[tokFile.Offset(lineStart):tokFile.Offset(l.index.Pos())]
	suffix := bytes.TrimRight(
		src[tokFile.Offset(l.index.End()):lineEnd],
		"\r\n",
	)
	replacement := l.name
	if l.two {
		replacement += ", " + l.okName
	}
	line := slices.Concat(prefix, []byte(replacement), suffix)
	return linelen.CheckLine(line)
}

// hoist returns the fix that declares every lookup of one loop above it and
// replaces the index expressions with the names the declarations bind.
func hoist(ls []*lookup) analysis.SuggestedFix {
	var (
		decls strings.Builder
		edits = []analysis.TextEdit{{
			Pos: ls[0].anchor.Pos(),
			End: ls[0].anchor.Pos(),
		}}
	)
	for _, l := range ls {
		if l.decl {
			decls.WriteString(l.name)
			if l.two {
				fmt.Fprintf(&decls, ", %s", l.okName)
			}
			fmt.Fprintf(
				&decls, " := %s[%s]\n",
				astexpr.Text(l.mapx), astexpr.Text(l.key),
			)
		}
		replacement := l.name
		if l.two {
			replacement += ", " + l.okName
		}
		edits = append(edits, analysis.TextEdit{
			Pos:     l.index.Pos(),
			End:     l.index.End(),
			NewText: []byte(replacement),
		})
	}
	edits[0].NewText = []byte(decls.String())
	return analysis.SuggestedFix{
		Message:   "Hoist the lookup before the loop",
		TextEdits: edits,
	}
}

// diagnostic returns the report for one lookup.
func diagnostic(l *lookup) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:     l.index.Pos(),
		End:     l.index.End(),
		Message: "map lookup does not depend on the loop; evaluate it once before the loop",
	}
}

// names returns the names that a declaration in file would shadow: every
// identifier written in the file and every name declared in the package.
func names(pass *analysis.Pass, file *ast.File) map[string]bool {
	used := make(map[string]bool)
	for _, name := range pass.Pkg.Scope().Names() {
		used[name] = true
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			used[ident.Name] = true
		}
		return true
	})
	return used
}

// pick returns a name that no existing identifier holds, and reserves it for
// the next call.
func pick(used map[string]bool, base string) string {
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name += strconv.Itoa(i)
		}
		if !used[name] {
			used[name] = true
			return name
		}
	}
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

// inScope reports whether every name in expr has the same object at pos, where
// the fix writes the declaration.
func inScope(pass *analysis.Pass, expr ast.Expr, pos token.Pos) bool {
	skip := make(map[*ast.Ident]struct{})
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			skip[n.Sel] = struct{}{}
		case *ast.CompositeLit:
			typ := pass.TypesInfo.TypeOf(n)
			if typ == nil {
				return true
			}
			if _, ok := typ.Underlying().(*types.Struct); !ok {
				return true
			}
			for _, elt := range n.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if ident, ok := ast.Unparen(kv.Key).(*ast.Ident); ok {
					skip[ident] = struct{}{}
				}
			}
		}
		return true
	})
	ok := true
	ast.Inspect(expr, func(n ast.Node) bool {
		ident, isIdent := n.(*ast.Ident)
		if !isIdent {
			return true
		}
		if _, ok := skip[ident]; ok {
			return true
		}
		obj := pass.TypesInfo.Uses[ident]
		if obj == nil || obj.Parent() == types.Universe {
			return true
		}
		if imports.Lookup(pass, ident.Name, pos) != obj {
			ok = false
		}
		return ok
	})
	return ok
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

// attached reports whether a comment that starts before pos ends on pos's
// line or the line above it, where the declaration would separate it from the
// loop.
func attached(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	line := pass.Fset.Position(pos).Line
	for _, group := range file.Comments {
		if group.Pos() >= pos {
			continue
		}
		end := pass.Fset.Position(group.End()).Line
		if end == line || end == line-1 {
			return true
		}
	}
	return false
}

// functionBody returns the body of the function that holds node.
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

// jumpedOver reports whether a goto in the function jumps from before anchor
// to a label at or after it, which Go rejects when anchor is a declaration.
func jumpedOver(parent map[ast.Node]ast.Node, anchor ast.Node) bool {
	body := functionBody(parent, anchor)
	if body == nil {
		return false
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
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if n.Tok != token.GOTO {
				return true
			}
			target, ok := labels[n.Label.Name]
			if !ok || target < anchor.Pos() {
				return true
			}
			if n.Pos() < anchor.Pos() {
				found = true
			}
			if n.Pos() > anchor.End() && target <= anchor.End() {
				found = true
			}
		}
		return !found
	})
	return found
}

// mapOf returns the map type of e.
func mapOf(pass *analysis.Pass, e ast.Expr) (*types.Map, bool) {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return nil, false
	}
	m, ok := t.Underlying().(*types.Map)
	return m, ok
}
