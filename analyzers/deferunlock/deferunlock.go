// Package deferunlock provides an analysis.Analyzer that reports a deferred
// Unlock without Lock, or a deferred RUnlock without RLock, in the same
// function.
//
// sync.Mutex.Unlock's documentation says "It is a run-time error if m is not
// locked on entry to Unlock", and sync.RWMutex.RUnlock's says the same for a
// read lock (https://pkg.go.dev/sync#Mutex.Unlock,
// https://pkg.go.dev/sync#RWMutex.RUnlock). The Defer statements section of
// the spec says a deferred call runs when the surrounding function returns
// (https://go.dev/ref/spec#Defer_statements). If the function has not taken
// the lock, the deferred call runs with no lock held. A sync.Mutex or
// sync.RWMutex release without a lock is a run-time error; another type with
// Unlock or RUnlock may not fail.
//
// A matching lock is a call to Lock on the receiver for Unlock, or to RLock
// for RUnlock, in the same function body. A TryLock or TryRLock counts unless
// the call is a bare expression statement or its result is assigned to the
// blank identifier. A call whose result is discarded inside another expression
// is also thrown away, unless it is passed to a call. A TryLock that returns
// false still counts; the result is not checked. The call may appear
// before or after the defer, or on a branch. A deferred Lock counts only when
// it is declared after the deferred release, so it runs first. A branch that
// may not run still counts, and a deferred literal whose body is a single lock
// call counts at the defer statement. A lock taken in a goroutine does not
// count.
//
// A pointer alias to a variable matches a lock on the original. An address-of
// alias follows the aliased value. An alias to a field or an index, or from a
// type assertion, is not followed. An assignment to a value base after the
// earlier of the lock and the release stops a match. An assignment to a
// pointer base between the two calls stops a match, because a deferred method
// value saves its receiver. Alias assignments are resolved at the position of
// the lock and the release. For a receiver behind an index or a type
// assertion, only an assignment between the two calls stops a match. An index
// expression is compared structurally; a change to the index variable between
// the calls is not tracked.
//
// Only calls in the function body count. A lock that the caller or a helper
// takes on the receiver is not a matching lock, and the release is reported.
// A receiver whose base value's last assignment before the defer came from a
// call is left alone, because the call may return with the lock held. A
// nested function literal is a separate function.
//
// Locks and releases are not paired, so a lock that another release also
// unlocks still matches. The README describes the calls that are checked and
// the ones that are left alone.
//
// The rule is adapted from staticcheck's SA5001 family of checks for deferred
// calls. SA5001 reports a deferred Close before the error from the open is
// checked.
//
// The rule matches by method name, so a type whose release is valid without a
// lock is reported.
//
// Report only: removing the defer and taking the lock earlier are both
// repairs, and they leave the function in different states.
package deferunlock

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "deferunlock",
	Doc:  "reports a deferred Unlock without Lock or a deferred RUnlock without RLock",
	Run:  run,
}

var (
	lockNames = []string{"Lock", "RLock"}
	tryNames  = []string{"TryLock", "TryRLock"}
	tryLocks  = map[string]string{"TryLock": "Lock", "TryRLock": "RLock"}
	releases  = map[string]string{"Unlock": "Lock", "RUnlock": "RLock"}
)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					check(pass, n.Body)
				}
			case *ast.FuncLit:
				check(pass, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the deferred releases in body that have no matching lock in
// the same function. A nested function literal is checked by the caller.
func check(pass *analysis.Pass, body *ast.BlockStmt) {
	var (
		origins                   = callOrigins(pass, body)
		aliases                   = aliasMap(pass, body)
		discarded                 = tryDiscards(pass, body)
		taken, deferred, released []*ast.SelectorExpr
	)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.GoStmt:
			for _, arg := range n.Call.Args {
				noteNow(pass, arg, &taken, discarded)
			}
			return false
		case *ast.DeferStmt:
			if sel, ok := releaseCall(pass, n.Call); ok {
				released = append(released, sel)
			}
			if sel, ok := lockCall(pass, n.Call, false); ok {
				deferred = append(deferred, sel)
			}
			if lit, ok := ast.Unparen(n.Call.Fun).(*ast.FuncLit); ok {
				if sel, ok := singleLock(pass, lit.Body); ok {
					deferred = append(deferred, sel)
				}
			}
			for _, arg := range n.Call.Args {
				noteNow(pass, arg, &taken, discarded)
			}
			return false
		case *ast.CallExpr:
			if discarded[n] {
				return true
			}
			if sel, ok := lockCall(pass, n, true); ok {
				taken = append(taken, sel)
			}
		}
		return true
	})
	for _, sel := range released {
		if skip(pass, taken, deferred, origins, aliases, sel) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos: sel.Sel.Pos(),
			End: sel.Sel.End(),
			Message: "deferred " + sel.Sel.Name +
				" has no matching " + releases[sel.Sel.Name] +
				" in this function",
		})
	}
}

func releaseCall(pass *analysis.Pass, call *ast.CallExpr) (*ast.SelectorExpr, bool) {
	sel, ok := methodCall(pass, call)
	if !ok {
		return nil, false
	}
	if _, ok := releases[sel.Sel.Name]; !ok {
		return nil, false
	}
	return sel, true
}

// try includes TryLock and TryRLock, whose result is used.
func lockCall(pass *analysis.Pass, call *ast.CallExpr, try bool) (*ast.SelectorExpr, bool) {
	sel, ok := methodCall(pass, call)
	if !ok {
		return nil, false
	}
	if slices.Contains(lockNames, sel.Sel.Name) {
		return sel, true
	}
	if try && slices.Contains(tryNames, sel.Sel.Name) {
		return sel, true
	}
	return nil, false
}

// singleLock returns the lock call in a body with one expression statement.
func singleLock(pass *analysis.Pass, body *ast.BlockStmt) (*ast.SelectorExpr, bool) {
	if len(body.List) != 1 {
		return nil, false
	}
	stmt, ok := body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	return lockCall(pass, call, false)
}

// methodCall returns the selector when call is a method with no arguments. A
// field of function type is not a method, and a call with an argument is not a
// lock or release method.
func methodCall(pass *analysis.Pass, call *ast.CallExpr) (*ast.SelectorExpr, bool) {
	if len(call.Args) != 0 {
		return nil, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return nil, false
	}
	return sel, true
}

func skip(pass *analysis.Pass, taken, deferred []*ast.SelectorExpr, origins map[types.Object][]origin, aliases aliasTable, sel *ast.SelectorExpr) bool {
	return hasLock(pass, taken, deferred, sel, aliases, origins) ||
		callReturned(pass, sel.X, origins, aliases, sel.Pos())
}

// A deferred lock matches only when it is declared after the release, so it
// runs first.
func hasLock(pass *analysis.Pass, taken, deferred []*ast.SelectorExpr, sel *ast.SelectorExpr, aliases aliasTable, origins map[types.Object][]origin) bool {
	for _, lock := range taken {
		if matches(pass, lock, sel, aliases, origins) {
			return true
		}
	}
	for _, lock := range deferred {
		if lock.Pos() <= sel.Pos() {
			continue
		}
		if matches(pass, lock, sel, aliases, origins) {
			return true
		}
	}
	return false
}

func matches(pass *analysis.Pass, lock, release *ast.SelectorExpr, aliases aliasTable, origins map[types.Object][]origin) bool {
	name := releases[release.Sel.Name]
	if lock.Sel.Name != name && tryLocks[lock.Sel.Name] != name {
		return false
	}
	if !samePath(pass, lock, release, aliases) {
		if !sameReceiver(pass, lock, release, aliases) {
			return false
		}
	}
	return !reassigned(pass, lock, release, aliases, origins)
}

// samePath reports whether lock and release reach the same field path from
// the same base object, including a method promoted from an embedded field.
func samePath(pass *analysis.Pass, lock, release *ast.SelectorExpr, aliases aliasTable) bool {
	lockBase, lockPath, ok := receiverPath(pass, lock)
	if !ok {
		return false
	}
	releaseBase, releasePath, ok := receiverPath(pass, release)
	if !ok {
		return false
	}
	return resolve(aliases, lockBase, lock.Pos()) ==
		resolve(aliases, releaseBase, release.Pos()) &&
		slices.Equal(lockPath, releasePath)
}

// receiverPath returns the base object and field path a method selector
// reaches, including the embedded fields a promoted method comes from.
func receiverPath(pass *analysis.Pass, sel *ast.SelectorExpr) (types.Object, []int, bool) {
	var (
		path          []int
		selection, ok = pass.TypesInfo.Selections[sel]
	)
	if ok && selection.Kind() == types.MethodVal {
		index := selection.Index()
		if len(index) > 0 {
			path = append(path, index[:len(index)-1]...)
		}
	}
	base, basePath, ok := exprPath(pass, sel.X)
	if !ok {
		return nil, nil, false
	}
	return base, append(basePath, path...), true
}

// exprPath returns the base object and field path of an expression.
func exprPath(pass *analysis.Pass, expr ast.Expr) (types.Object, []int, bool) {
	expr = strip(expr)
	switch expr := expr.(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.ObjectOf(expr)
		return obj, nil, obj != nil
	case *ast.SelectorExpr:
		selection, ok := pass.TypesInfo.Selections[expr]
		if !ok {
			obj := pass.TypesInfo.ObjectOf(expr.Sel)
			return obj, nil, obj != nil
		}
		if selection.Kind() != types.FieldVal {
			return nil, nil, false
		}
		base, path, ok := exprPath(pass, expr.X)
		if !ok {
			return nil, nil, false
		}
		return base, append(path, selection.Index()...), true
	}
	return nil, nil, false
}

// sameReceiver reports whether the receivers of lock and release match.
func sameReceiver(pass *analysis.Pass, lock, release *ast.SelectorExpr, aliases aliasTable) bool {
	var (
		a, b       = lock.X, release.X
		aPos, bPos = lock.Pos(), release.Pos()
	)
	return sameReceiverExpr(pass, a, b, aliases, aPos, bPos)
}

// sameReceiverExpr reports whether a and b read the same receiver. Identifiers
// and fields are compared by object, with parentheses, address-of expressions,
// and dereferences ignored at every level. Pointer aliases are resolved at the
// position of each side. Type assertions, indexes, and unary and binary
// expressions are compared structurally.
func sameReceiverExpr(pass *analysis.Pass, a, b ast.Expr, aliases aliasTable, aPos, bPos token.Pos) bool {
	a, b = strip(a), strip(b)
	switch a := a.(type) {
	case *ast.Ident:
		return sameIdent(pass, a, b, aliases, aPos, bPos)
	case *ast.SelectorExpr:
		return sameSelector(pass, a, b, aliases, aPos, bPos)
	case *ast.IndexExpr:
		return sameIndex(pass, a, b, aliases, aPos, bPos)
	case *ast.TypeAssertExpr:
		return sameAssert(pass, a, b, aliases, aPos, bPos)
	}
	return sameOperand(pass, a, b, aliases, aPos, bPos)
}

// sameIdent reports whether a and b name the same object after aliases.
func sameIdent(pass *analysis.Pass, a *ast.Ident, b ast.Expr, aliases aliasTable, aPos, bPos token.Pos) bool {
	ident, ok := b.(*ast.Ident)
	if !ok {
		return false
	}
	aObj := pass.TypesInfo.ObjectOf(a)
	bObj := pass.TypesInfo.ObjectOf(ident)
	return aObj != nil && bObj != nil &&
		resolve(aliases, aObj, aPos) == resolve(aliases, bObj, bPos)
}

// sameSelector reports whether a and b select the same field and receiver.
func sameSelector(pass *analysis.Pass, a *ast.SelectorExpr, b ast.Expr, aliases aliasTable, aPos, bPos token.Pos) bool {
	sel, ok := b.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if pass.TypesInfo.ObjectOf(a.Sel) != pass.TypesInfo.ObjectOf(sel.Sel) {
		return false
	}
	return sameReceiverExpr(pass, a.X, sel.X, aliases, aPos, bPos)
}

// sameIndex reports whether a and b index the same receiver and index.
func sameIndex(pass *analysis.Pass, a *ast.IndexExpr, b ast.Expr, aliases aliasTable, aPos, bPos token.Pos) bool {
	index, ok := b.(*ast.IndexExpr)
	return ok && sameReceiverExpr(pass, a.X, index.X, aliases, aPos, bPos) &&
		sameReceiverExpr(pass, a.Index, index.Index, aliases, aPos, bPos)
}

// sameAssert reports whether a and b assert the same type and receiver.
func sameAssert(pass *analysis.Pass, a *ast.TypeAssertExpr, b ast.Expr, aliases aliasTable, aPos, bPos token.Pos) bool {
	expr, ok := b.(*ast.TypeAssertExpr)
	if !ok {
		return false
	}
	at, bt := pass.TypesInfo.TypeOf(a), pass.TypesInfo.TypeOf(expr)
	if !types.Identical(at, bt) {
		return false
	}
	return sameReceiverExpr(pass, a.X, expr.X, aliases, aPos, bPos)
}

// sameOperand reports whether a and b are the same unary or binary expression,
// or another expression that astexpr.Same handles.
func sameOperand(pass *analysis.Pass, a, b ast.Expr, aliases aliasTable, aPos, bPos token.Pos) bool {
	switch a := a.(type) {
	case *ast.BinaryExpr:
		expr, ok := b.(*ast.BinaryExpr)
		if !ok || a.Op != expr.Op {
			return false
		}
		return sameReceiverExpr(pass, a.X, expr.X, aliases, aPos, bPos) &&
			sameReceiverExpr(pass, a.Y, expr.Y, aliases, aPos, bPos)
	case *ast.UnaryExpr:
		expr, ok := b.(*ast.UnaryExpr)
		if !ok || a.Op != expr.Op || a.Op == token.ARROW {
			return false
		}
		return sameReceiverExpr(pass, a.X, expr.X, aliases, aPos, bPos)
	}
	return astexpr.Same(pass, a, b)
}

// strip removes parentheses, address-of expressions, and dereferences.
func strip(expr ast.Expr) ast.Expr {
	expr = ast.Unparen(expr)
	switch expr := expr.(type) {
	case *ast.UnaryExpr:
		if expr.Op == token.AND {
			return strip(expr.X)
		}
	case *ast.StarExpr:
		return strip(expr.X)
	}
	return expr
}

// tryDiscards returns the TryLock and TryRLock calls whose result is thrown
// away.
func tryDiscards(pass *analysis.Pass, body *ast.BlockStmt) map[*ast.CallExpr]bool {
	discarded := map[*ast.CallExpr]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ExprStmt:
			noteDiscarded(pass, n.X, discarded)
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if !isBlank(lhs) || i >= len(n.Rhs) {
					continue
				}
				noteDiscarded(pass, n.Rhs[i], discarded)
			}
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if !isBlank(name) || i >= len(n.Values) {
					continue
				}
				noteDiscarded(pass, n.Values[i], discarded)
			}
		}
		return true
	})
	return discarded
}

// noteDiscarded records the TryLock calls whose result is thrown away. A
// negation or a conversion keeps the result thrown away; a call to another
// function may use it.
func noteDiscarded(pass *analysis.Pass, expr ast.Expr, discarded map[*ast.CallExpr]bool) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		if isTryCall(pass, expr) {
			discarded[expr] = true
			return
		}
		if isConversion(pass, expr) && len(expr.Args) == 1 {
			noteDiscarded(pass, expr.Args[0], discarded)
		}
	case *ast.UnaryExpr:
		noteDiscarded(pass, expr.X, discarded)
	case *ast.BinaryExpr:
		noteDiscarded(pass, expr.X, discarded)
		noteDiscarded(pass, expr.Y, discarded)
	case *ast.IndexExpr:
		noteDiscarded(pass, expr.X, discarded)
		noteDiscarded(pass, expr.Index, discarded)
	case *ast.SliceExpr:
		noteDiscarded(pass, expr.X, discarded)
	case *ast.CompositeLit:
		for _, elt := range expr.Elts {
			noteDiscarded(pass, elt, discarded)
		}
	case *ast.KeyValueExpr:
		noteDiscarded(pass, expr.Key, discarded)
		noteDiscarded(pass, expr.Value, discarded)
	case *ast.StarExpr:
		noteDiscarded(pass, expr.X, discarded)
	case *ast.TypeAssertExpr:
		noteDiscarded(pass, expr.X, discarded)
	}
}

// noteNow records the locks in expr, which is evaluated at the defer or go
// statement.
func noteNow(pass *analysis.Pass, expr ast.Expr, taken *[]*ast.SelectorExpr, discarded map[*ast.CallExpr]bool) {
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if discarded[n] {
				return true
			}
			if sel, ok := lockCall(pass, n, true); ok {
				*taken = append(*taken, sel)
			}
		}
		return true
	})
}

func isTryCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := methodCall(pass, call)
	return ok && slices.Contains(tryNames, sel.Sel.Name)
}

func isConversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	tv, ok := pass.TypesInfo.Types[call.Fun]
	return ok && tv.IsType()
}

func isBlank(expr ast.Expr) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && id.Name == "_"
}

type aliasOrigin struct {
	pos     token.Pos
	target  types.Object
	address bool
}

type aliasTable map[types.Object][]aliasOrigin

func aliasMap(pass *analysis.Pass, body *ast.BlockStmt) aliasTable {
	var (
		aliases = aliasTable{}
		posts   = forPosts(body)
	)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			pos := n.Pos()
			if post, ok := posts[n]; ok {
				pos = post
			}
			noteAliases(pass, n.Lhs, n.Rhs, pos, aliases)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(n.Names))
			for i, name := range n.Names {
				lhs[i] = name
			}
			noteAliases(pass, lhs, n.Values, n.Pos(), aliases)
		}
		return true
	})
	return aliases
}

func noteAliases(pass *analysis.Pass, lhs, rhs []ast.Expr, pos token.Pos, aliases aliasTable) {
	if len(rhs) != len(lhs) {
		return
	}
	for i := range rhs {
		id, ok := ast.Unparen(lhs[i]).(*ast.Ident)
		if !ok {
			continue
		}
		obj := pass.TypesInfo.ObjectOf(id)
		if obj == nil || !isPointer(pass.TypesInfo.TypeOf(id)) {
			continue
		}
		target, address, ok := aliasTarget(pass, rhs[i])
		if !ok {
			target = nil
		}
		aliases[obj] = append(aliases[obj], aliasOrigin{pos, target, address})
	}
}

// aliasTarget returns the object that expr aliases and whether expr takes its
// address.
func aliasTarget(pass *analysis.Pass, expr ast.Expr) (obj types.Object, address, ok bool) {
	expr = ast.Unparen(expr)
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		if unary.Op != token.AND {
			return nil, false, false
		}
		address = true
		expr = ast.Unparen(unary.X)
	}
	id, ok := expr.(*ast.Ident)
	if !ok {
		return nil, false, false
	}
	obj = pass.TypesInfo.ObjectOf(id)
	return obj, address, obj != nil
}

func isPointer(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Pointer)
	return ok
}

func lastAlias(aliases aliasTable, obj types.Object, pos token.Pos) (aliasOrigin, bool) {
	var last *aliasOrigin
	for i := range aliases[obj] {
		a := &aliases[obj][i]
		if a.pos < pos && (last == nil || a.pos > last.pos) {
			last = a
		}
	}
	if last == nil {
		return aliasOrigin{}, false
	}
	return *last, true
}

func resolve(aliases aliasTable, obj types.Object, pos token.Pos) types.Object {
	for range len(aliases) + 1 {
		last, ok := lastAlias(aliases, obj, pos)
		if !ok || last.target == nil || last.target == obj {
			break
		}
		obj = last.target
		pos = last.pos
	}
	return obj
}

// addressTarget returns the value an address-of alias chain points at.
func addressTarget(aliases aliasTable, obj types.Object, pos token.Pos) (types.Object, bool) {
	var address bool
	for range len(aliases) + 1 {
		last, ok := lastAlias(aliases, obj, pos)
		if !ok || last.target == nil {
			break
		}
		address = address || last.address
		obj = last.target
		pos = last.pos
	}
	if address {
		return obj, true
	}
	return nil, false
}

func addressAlias(aliases aliasTable, obj types.Object, pos token.Pos) bool {
	_, ok := addressTarget(aliases, obj, pos)
	return ok
}

// reassigned reports whether an assignment to the receiver invalidates the
// match. A value base is checked after the earlier call; a pointer or indexed
// base only between the two calls.
func reassigned(pass *analysis.Pass, lock, release *ast.SelectorExpr, aliases aliasTable, origins map[types.Object][]origin) bool {
	var (
		base, _, ok = receiverPath(pass, release)
		betweenOnly = !ok
	)
	if !ok {
		base, ok = rootObject(pass, release.X)
	}
	if !ok {
		return false
	}
	if addressAlias(aliases, base, release.Pos()) {
		base = resolve(aliases, base, release.Pos())
		betweenOnly = false
	}
	return reassignedBase(base, lock, release, origins, betweenOnly)
}

// rootObject returns the base object of a receiver expression, including one
// behind an index or a type assertion.
func rootObject(pass *analysis.Pass, expr ast.Expr) (types.Object, bool) {
	switch expr := strip(expr).(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.ObjectOf(expr)
		return obj, obj != nil
	case *ast.SelectorExpr:
		return rootObject(pass, expr.X)
	case *ast.IndexExpr:
		return rootObject(pass, expr.X)
	case *ast.TypeAssertExpr:
		return rootObject(pass, expr.X)
	}
	return nil, false
}

// reassignedBase reports whether base is assigned after the earlier of the
// lock and the release. Only an assignment between the two calls stops a match
// for a pointer base, because a deferred method value saves its receiver.
func reassignedBase(base types.Object, lock, release *ast.SelectorExpr, origins map[types.Object][]origin, betweenOnly bool) bool {
	if betweenOnly || isPointer(base.Type()) {
		lo, hi := lock.Pos(), release.Pos()
		if lo > hi {
			lo, hi = hi, lo
		}
		for _, o := range origins[base] {
			if o.pos > lo && o.pos < hi {
				return true
			}
		}
		return false
	}
	start := lock.Pos()
	if release.Pos() < start {
		start = release.Pos()
	}
	for _, o := range origins[base] {
		if o.pos > start {
			return true
		}
	}
	return false
}

// origin is one assignment to a name, with the position where it runs and
// whether the value came from a call.
type origin struct {
	pos  token.Pos
	call bool
}

func callOrigins(pass *analysis.Pass, body *ast.BlockStmt) map[types.Object][]origin {
	var (
		origins = map[types.Object][]origin{}
		posts   = forPosts(body)
	)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			pos := n.Pos()
			if post, ok := posts[n]; ok {
				pos = post
			}
			noteOrigins(pass, n.Lhs, n.Rhs, pos, origins)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(n.Names))
			for i, name := range n.Names {
				lhs[i] = name
			}
			noteOrigins(pass, lhs, n.Values, n.Pos(), origins)
		case *ast.RangeStmt:
			fromCall := callResult(pass, n.X) ||
				callReturned(pass, n.X, origins, nil, n.Pos())
			noteIdent(pass, n.Key, n.Pos(), fromCall, origins)
			noteIdent(pass, n.Value, n.Pos(), fromCall, origins)
		}
		return true
	})
	return origins
}

// forPosts returns the position after each loop body for the assignment in
// the loop's post statement, which runs after the body.
func forPosts(body *ast.BlockStmt) map[*ast.AssignStmt]token.Pos {
	posts := map[*ast.AssignStmt]token.Pos{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ForStmt:
			if assign, ok := n.Post.(*ast.AssignStmt); ok {
				posts[assign] = n.Body.End()
			}
		}
		return true
	})
	return posts
}

func noteOrigins(pass *analysis.Pass, lhs, rhs []ast.Expr, pos token.Pos, origins map[types.Object][]origin) {
	if len(rhs) == 1 {
		fromCall := callResult(pass, rhs[0]) ||
			callReturned(pass, rhs[0], origins, nil, pos)
		for _, expr := range lhs {
			noteIdent(pass, expr, pos, fromCall, origins)
		}
		return
	}
	if len(rhs) != len(lhs) {
		return
	}
	fromCall := make([]bool, len(rhs))
	for i := range rhs {
		fromCall[i] = callResult(pass, rhs[i]) ||
			callReturned(pass, rhs[i], origins, nil, pos)
	}
	for i := range rhs {
		noteIdent(pass, lhs[i], pos, fromCall[i], origins)
	}
}

func noteIdent(pass *analysis.Pass, expr ast.Expr, pos token.Pos, fromCall bool, origins map[types.Object][]origin) {
	if expr == nil {
		return
	}
	id, ok := ast.Unparen(expr).(*ast.Ident)
	if !ok {
		return
	}
	obj := pass.TypesInfo.ObjectOf(id)
	if obj == nil {
		return
	}
	origins[obj] = append(origins[obj], origin{pos, fromCall})
}

// callResult reports whether expr is a call to a function, or a field, index,
// dereference, address, or type assertion on one. A conversion and a builtin
// such as new or make are not calls to a function that can return a locked
// value.
func callResult(pass *analysis.Pass, expr ast.Expr) bool {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		if isConversion(pass, expr) {
			return false
		}
		if id, ok := ast.Unparen(expr.Fun).(*ast.Ident); ok {
			if _, ok := pass.TypesInfo.Uses[id].(*types.Builtin); ok {
				return false
			}
		}
		return true
	case *ast.SelectorExpr:
		return callResult(pass, expr.X)
	case *ast.IndexExpr:
		return callResult(pass, expr.X)
	case *ast.StarExpr:
		return callResult(pass, expr.X)
	case *ast.UnaryExpr:
		return expr.Op == token.AND && callResult(pass, expr.X)
	case *ast.TypeAssertExpr:
		return callResult(pass, expr.X)
	}
	return false
}

// callReturnedObject reports whether obj receives a call result at pos. An
// address-of alias follows the target, whose value can change after the alias.
func callReturnedObject(pass *analysis.Pass, obj types.Object, origins map[types.Object][]origin, aliases aliasTable, pos token.Pos) bool {
	if obj == nil {
		return false
	}
	if target, ok := addressTarget(aliases, obj, pos); ok {
		return callReturnedObject(pass, target, origins, aliases, pos)
	}
	var last *origin
	for i := range origins[obj] {
		o := &origins[obj][i]
		if o.pos < pos && (last == nil || o.pos > last.pos) {
			last = o
		}
	}
	return last != nil && last.call
}

// callReturned reports whether the last assignment before pos gave expr a
// call result.
func callReturned(pass *analysis.Pass, expr ast.Expr, origins map[types.Object][]origin, aliases aliasTable, pos token.Pos) bool {
	switch expr := receiver(expr).(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.ObjectOf(expr)
		return callReturnedObject(pass, obj, origins, aliases, pos)
	case *ast.SelectorExpr:
		return callReturned(pass, expr.X, origins, aliases, pos)
	case *ast.IndexExpr:
		return callReturned(pass, expr.X, origins, aliases, pos)
	case *ast.TypeAssertExpr:
		return callReturned(pass, expr.X, origins, aliases, pos)
	case *ast.CallExpr:
		return callResult(pass, expr)
	}
	return false
}

func receiver(expr ast.Expr) ast.Expr {
	expr = ast.Unparen(expr)
	switch expr := expr.(type) {
	case *ast.UnaryExpr:
		if expr.Op == token.AND {
			return receiver(expr.X)
		}
	case *ast.StarExpr:
		return receiver(expr.X)
	}
	return expr
}
