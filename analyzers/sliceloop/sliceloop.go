// Package sliceloop provides an analysis.Analyzer that reports a loop that
// copies a source slice into another slice one element at a time.
//
// The section of the spec on appending to and copying slices defines copy,
// which copies the elements of src into dst and returns the number copied
// (https://go.dev/ref/spec#Appending_and_copying_slices). The loop ranges over
// the source, or counts from zero to len(src), and its body assigns one
// element to the other slice at the same index. In the slice forms, both
// expressions must be slices or type parameters over slices, with identical
// element types. A counted loop may also copy a string into a byte slice,
// including through a type parameter over strings and byte slices. The counted
// form must test i < len(src) or len(src) > i; another exit test is left
// alone. Its post may be i++, i += 1, i = i + 1, or i = 1 + i.
//
// The rule matches a loop whose body is one assignment of the source element
// to the target at the same index. The rule does not call copy, so a shadowed
// copy does not change the report. It leaves alone a loop whose key is
// assigned in the header instead of declared there with :=, a target or source
// expression that reads the key, the value, or the other expression, an
// expression that can run code when evaluated, whose effects can change the
// slices, a target expression that may allocate new storage each pass, as a
// slice or array literal or a conversion from a string, and a loop whose
// target and source are the same expression. It also leaves alone a loop whose
// iteration count comes from the target, following the source-counted form in
// S1001. In the slice forms, the rule covers loops whose source and target are
// slices or type parameters over slices; a loop whose source or target has an
// array type, a pointer to an array, or a type parameter over either is
// outside that scope. A loop over a prefix of the source whose body indexes
// the base is left alone, since the loop can read past the base length. A loop
// over a map is left alone, as is a range loop over a string.
//
// Report only: copy stops at the shorter length, and the loop panics when the
// target is shorter than the source. When the target starts after the source
// in the same array, the loop can overwrite source elements it has not read
// yet. The spec allows the source and destination to overlap, so copy can
// produce a different result. A loop that runs zero times does not evaluate
// the target, and copy does.
//
// The rule is adapted from staticcheck's S1001. S1001 also reports loops over
// arrays; this rule handles slices.
//
//	func duplicate(dst, src []string) {
//		for i, s := range src {
//			dst[i] = s
//		}
//	}
package sliceloop

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "sliceloop",
	Doc:  "reports a loop that copies one slice into another",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.RangeStmt:
				checkRange(pass, n)
			case *ast.ForStmt:
				checkFor(pass, n)
			}
			return true
		})
	}
	return nil, nil
}

// checkRange reports a range loop that copies its source into another slice.
func checkRange(pass *analysis.Pass, loop *ast.RangeStmt) {
	// A key assigned with = is written by the loop; copy would leave it alone.
	if loop.Tok != token.DEFINE {
		return
	}
	key, ok := loop.Key.(*ast.Ident)
	if !ok {
		return
	}
	if len(loop.Body.List) != 1 {
		return
	}
	target, rhs, ok := assignment(pass, loop.Body.List[0], key)
	if !ok {
		return
	}
	source := ast.Unparen(loop.X)
	value, ok := matched(pass, loop, rhs, source, key)
	if !ok {
		return
	}
	keyObj := pass.TypesInfo.ObjectOf(key)
	if !independent(pass, target, source, keyObj, value) {
		return
	}
	if astexpr.Effectful(pass, target) || astexpr.Effectful(pass, source) {
		return
	}
	if sameSource(pass, target, source) {
		return
	}
	if !copies(pass, target, source) {
		return
	}
	report(pass, loop)
}

// independent reports whether target and source are free of the loop's key and
// value and of each other, so the write to target cannot change the slice that
// source reads.
func independent(pass *analysis.Pass, target, source ast.Expr, objs ...types.Object) bool {
	if mentioned(pass, target, objs...) || mentioned(pass, source, objs...) {
		return false
	}
	return !mentionsSource(pass, target, source) &&
		!mentionsSource(pass, source, target)
}

// mentionsSource reports whether e contains an expression that reads source.
func mentionsSource(pass *analysis.Pass, e ast.Node, source ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if expr, ok := n.(ast.Expr); ok && sameSource(pass, expr, source) {
			found = true
		}
		return !found
	})
	return found
}

// matched returns the range value a loop declares, and reports whether rhs
// reads the source at key or that value.
func matched(pass *analysis.Pass, loop *ast.RangeStmt, rhs, source ast.Expr, key *ast.Ident) (value types.Object, ok bool) {
	if loop.Value == nil || blank(loop.Value) {
		return nil, indexed(pass, rhs, source, key)
	}
	ident, ok := loop.Value.(*ast.Ident)
	if !ok || !astexpr.Same(pass, rhs, ident) {
		return nil, false
	}
	return pass.TypesInfo.ObjectOf(ident), true
}

// checkFor reports a three-clause loop from zero to len(src) that copies its
// source into another slice.
func checkFor(pass *analysis.Pass, loop *ast.ForStmt) {
	key, ok := counter(pass, loop)
	if !ok {
		return
	}
	source, ok := bound(pass, loop.Cond, key)
	if !ok {
		return
	}
	if !incremented(pass, loop.Post, key) {
		return
	}
	if len(loop.Body.List) != 1 {
		return
	}
	target, rhs, ok := assignment(pass, loop.Body.List[0], key)
	if !ok || !indexed(pass, rhs, source, key) {
		return
	}
	keyObj := pass.TypesInfo.ObjectOf(key)
	if !independent(pass, target, source, keyObj) {
		return
	}
	if astexpr.Effectful(pass, target) || astexpr.Effectful(pass, source) {
		return
	}
	if sameSource(pass, target, source) {
		return
	}
	if !copies(pass, target, source) && !stringCopy(pass, target, source) {
		return
	}
	report(pass, loop)
}

// counter returns the key of a three-clause loop that starts at zero.
func counter(pass *analysis.Pass, loop *ast.ForStmt) (*ast.Ident, bool) {
	init, ok := loop.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE {
		return nil, false
	}
	if len(init.Lhs) != 1 || len(init.Rhs) != 1 || !zero(pass, init.Rhs[0]) {
		return nil, false
	}
	key, ok := init.Lhs[0].(*ast.Ident)
	return key, ok
}

// bound returns the source of a condition that compares key with len(source).
func bound(pass *analysis.Pass, cond ast.Expr, key *ast.Ident) (ast.Expr, bool) {
	bin, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok {
		return nil, false
	}
	switch bin.Op {
	case token.LSS:
		if !astexpr.Same(pass, bin.X, key) {
			return nil, false
		}
		return lenSource(pass, bin.Y)
	case token.GTR:
		if !astexpr.Same(pass, bin.Y, key) {
			return nil, false
		}
		return lenSource(pass, bin.X)
	}
	return nil, false
}

// lenSource returns the operand of a call to len.
func lenSource(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis != token.NoPos {
		return nil, false
	}
	if !builtin(pass, call.Fun, "len") {
		return nil, false
	}
	return ast.Unparen(call.Args[0]), true
}

// incremented reports whether post adds one to key.
func incremented(pass *analysis.Pass, post ast.Stmt, key *ast.Ident) bool {
	switch post := post.(type) {
	case *ast.IncDecStmt:
		return post.Tok == token.INC && astexpr.Same(pass, post.X, key)
	case *ast.AssignStmt:
		return increments(pass, post, key)
	}
	return false
}

// increments reports whether assign adds one to key.
func increments(pass *analysis.Pass, assign *ast.AssignStmt, key *ast.Ident) bool {
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	if !astexpr.Same(pass, assign.Lhs[0], key) {
		return false
	}
	switch assign.Tok {
	case token.ADD_ASSIGN:
		return one(pass, assign.Rhs[0])
	case token.ASSIGN:
		return addsOne(pass, assign.Rhs[0], key)
	}
	return false
}

// addsOne reports whether e is key + 1 or 1 + key.
func addsOne(pass *analysis.Pass, e ast.Expr, key *ast.Ident) bool {
	bin, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || bin.Op != token.ADD {
		return false
	}
	if astexpr.Same(pass, bin.X, key) && one(pass, bin.Y) {
		return true
	}
	return astexpr.Same(pass, bin.Y, key) && one(pass, bin.X)
}

// assignment returns the target of an index write and the value written to it.
func assignment(pass *analysis.Pass, stmt ast.Stmt, key *ast.Ident) (target, value ast.Expr, ok bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	index, ok := ast.Unparen(assign.Lhs[0]).(*ast.IndexExpr)
	if !ok || !astexpr.Same(pass, index.Index, key) {
		return nil, nil, false
	}
	target = ast.Unparen(index.X)
	if !stableTarget(pass, target) {
		return nil, nil, false
	}
	value = ast.Unparen(assign.Rhs[0])
	return target, value, true
}

// stableTarget reports whether target's storage is known to persist across
// passes.
func stableTarget(pass *analysis.Pass, target ast.Expr) bool {
	stable := true
	ast.Inspect(target, func(n ast.Node) bool {
		if !stable {
			return false
		}
		switch n := n.(type) {
		case *ast.CompositeLit:
			if allocating(pass.TypesInfo.TypeOf(n)) {
				stable = false
				return false
			}
		case *ast.CallExpr:
			if conversion(pass, n) {
				if len(n.Args) != 1 {
					stable = false
					return false
				}
				if _, ok := element(pass.TypesInfo.TypeOf(n.Args[0])); !ok {
					stable = false
					return false
				}
				break
			}
			if !astexpr.Pure(pass, n) {
				stable = false
				return false
			}
		}
		return true
	})
	return stable
}

// allocating reports whether t is a slice, an array, or a type parameter over
// either.
func allocating(t types.Type) bool {
	if _, ok := element(t); ok {
		return true
	}
	return astexpr.IsArray(t)
}

// indexed reports whether e reads source at key.
func indexed(pass *analysis.Pass, e, source ast.Expr, key *ast.Ident) bool {
	index, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok {
		return false
	}
	if !astexpr.Same(pass, index.Index, key) {
		return false
	}
	return sameSource(pass, index.X, source)
}

// sameSource reports whether a and b read the same slice expression. It adds
// slice expressions to the forms astexpr.Same compares.
func sameSource(pass *analysis.Pass, a, b ast.Expr) bool {
	a, b = unslice(pass, ast.Unparen(a)), unslice(pass, ast.Unparen(b))
	if astexpr.Same(pass, a, b) {
		return true
	}
	if ac, ok := a.(*ast.CallExpr); ok {
		if bc, ok := b.(*ast.CallExpr); ok && sameCalls(pass, ac, bc) {
			return true
		}
	}
	if sameStrip(pass, a, b) || sameStrip(pass, b, a) {
		return true
	}
	switch a := a.(type) {
	case *ast.TypeAssertExpr:
		return sameAssert(pass, a, b)
	case *ast.BinaryExpr:
		return sameBinary(pass, a, b)
	case *ast.UnaryExpr:
		return sameUnary(pass, a, b)
	case *ast.IndexExpr:
		return sameIndex(pass, a, b)
	}
	as, ok := a.(*ast.SliceExpr)
	if !ok {
		return false
	}
	bs, ok := b.(*ast.SliceExpr)
	if !ok {
		return false
	}
	return sameSlice(pass, as, bs)
}

// sameSlice reports whether a and b read the same slice expression.
func sameSlice(pass *analysis.Pass, a, b *ast.SliceExpr) bool {
	return a.Slice3 == b.Slice3 && sameLow(pass, a.Low, b.Low) &&
		sameOptional(pass, a.High, b.High) &&
		sameOptional(pass, a.Max, b.Max) &&
		sameSource(pass, a.X, b.X)
}

// sameLow reports whether a and b are both absent or zero, or read the same
// expression.
func sameLow(pass *analysis.Pass, a, b ast.Expr) bool {
	if a == nil {
		return b == nil || zero(pass, b)
	}
	if b == nil {
		return zero(pass, a)
	}
	return sameSource(pass, a, b)
}

// sameOptional reports whether a and b are both absent or read the same
// expression.
func sameOptional(pass *analysis.Pass, a, b ast.Expr) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameSource(pass, a, b)
}

// sameBinary reports whether b is the same binary expression as a.
func sameBinary(pass *analysis.Pass, a *ast.BinaryExpr, b ast.Expr) bool {
	other, ok := b.(*ast.BinaryExpr)
	return ok && a.Op == other.Op &&
		sameSource(pass, a.X, other.X) && sameSource(pass, a.Y, other.Y)
}

// sameUnary reports whether b is the same unary expression as a.
func sameUnary(pass *analysis.Pass, a *ast.UnaryExpr, b ast.Expr) bool {
	other, ok := b.(*ast.UnaryExpr)
	return ok && a.Op == other.Op && sameSource(pass, a.X, other.X)
}

// sameIndex reports whether b is the same index expression as a.
func sameIndex(pass *analysis.Pass, a *ast.IndexExpr, b ast.Expr) bool {
	other, ok := b.(*ast.IndexExpr)
	return ok && sameSource(pass, a.X, other.X) &&
		sameSource(pass, a.Index, other.Index)
}

// unslice returns e without a slice expression that reads the whole slice,
// such as x[:], x[0:], or x[:len(x)].
func unslice(pass *analysis.Pass, e ast.Expr) ast.Expr {
	for {
		slice, ok := ast.Unparen(e).(*ast.SliceExpr)
		if !ok || slice.Slice3 || slice.Max != nil {
			return e
		}
		if slice.Low != nil && !zero(pass, slice.Low) {
			return e
		}
		if slice.High != nil {
			source, ok := lenSource(pass, slice.High)
			if !ok || !sameSource(pass, source, slice.X) {
				return e
			}
		}
		e = ast.Unparen(slice.X)
	}
}

// sameCalls reports whether a and b are the same pure call.
func sameCalls(pass *analysis.Pass, a, b *ast.CallExpr) bool {
	return sameCall(pass, a, b) || samePure(pass, a, b)
}

// sameCall reports whether a and b are conversions of the same type over the
// same operand.
func sameCall(pass *analysis.Pass, a, b *ast.CallExpr) bool {
	if !conversion(pass, a) || !conversion(pass, b) {
		return false
	}
	if len(a.Args) != 1 || len(b.Args) != 1 {
		return false
	}
	var (
		aType = pass.TypesInfo.TypeOf(a)
		bType = pass.TypesInfo.TypeOf(b)
	)
	return types.Identical(aType, bType) &&
		sameSource(pass, a.Args[0], b.Args[0])
}

// samePure reports whether a and b are calls to the same pure function over
// the same arguments.
func samePure(pass *analysis.Pass, a, b *ast.CallExpr) bool {
	if conversion(pass, a) || conversion(pass, b) {
		return false
	}
	if !astexpr.Pure(pass, a) || !astexpr.Pure(pass, b) {
		return false
	}
	if len(a.Args) != len(b.Args) || !astexpr.Same(pass, a.Fun, b.Fun) {
		return false
	}
	for i := range a.Args {
		if !sameSource(pass, a.Args[i], b.Args[i]) {
			return false
		}
	}
	return true
}

// sameStrip reports whether e is a conversion over the same source as other,
// with the same indexed element type.
func sameStrip(pass *analysis.Pass, e, other ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || !conversion(pass, call) || !sameIndexed(pass, call, other) {
		return false
	}
	return sameSource(pass, call.Args[0], other)
}

// sameIndexed reports whether a and b yield the same element type when they
// are indexed.
func sameIndexed(pass *analysis.Pass, a, b ast.Expr) bool {
	ae, ok := indexedElem(pass.TypesInfo.TypeOf(a))
	if !ok {
		return false
	}
	be, ok := indexedElem(pass.TypesInfo.TypeOf(b))
	return ok && types.Identical(ae, be)
}

// indexedElem returns the element type of a slice, a string, or a type
// parameter over either.
func indexedElem(t types.Type) (types.Type, bool) {
	if elem, ok := element(t); ok {
		return elem, true
	}
	if byteSource(t) {
		return types.Typ[types.Byte], true
	}
	return nil, false
}

// conversion reports whether call is a type conversion.
func conversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	tv, ok := pass.TypesInfo.Types[call.Fun]
	return ok && tv.IsType()
}

// sameAssert reports whether b is the same type assertion as a.
func sameAssert(pass *analysis.Pass, a *ast.TypeAssertExpr, b ast.Expr) bool {
	other, ok := b.(*ast.TypeAssertExpr)
	if !ok {
		return false
	}
	aType := pass.TypesInfo.TypeOf(a)
	bType := pass.TypesInfo.TypeOf(other)
	return types.Identical(aType, bType) && sameSource(pass, a.X, other.X)
}

// zero reports whether e is the constant zero.
func zero(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	return ok && numeric(tv.Value) && constant.Sign(tv.Value) == 0
}

// one reports whether e is the constant one.
func one(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	return ok && numeric(tv.Value) &&
		constant.Compare(tv.Value, token.EQL, constant.MakeInt64(1))
}

// numeric reports whether v is an integer or floating-point constant.
func numeric(v constant.Value) bool {
	if v == nil {
		return false
	}
	return v.Kind() == constant.Int || v.Kind() == constant.Float
}

// blank reports whether e is the blank identifier.
func blank(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// builtin reports whether e is an identifier for the predeclared name.
func builtin(pass *analysis.Pass, e ast.Expr, name string) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup(name)
}

// mentioned reports whether e reads one of objs.
func mentioned(pass *analysis.Pass, e ast.Expr, objs ...types.Object) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := pass.TypesInfo.ObjectOf(id)
		for _, want := range objs {
			if obj != nil && obj == want {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// copies reports whether copy(target, source) type-checks. They must be slices
// or type parameters over slices, with identical element types.
func copies(pass *analysis.Pass, target, source ast.Expr) bool {
	to, ok := element(pass.TypesInfo.TypeOf(target))
	if !ok {
		return false
	}
	from, ok := element(pass.TypesInfo.TypeOf(source))
	return ok && types.Identical(to, from)
}

// stringCopy reports whether copy(target, source) copies a string or a byte
// slice into a byte slice.
func stringCopy(pass *analysis.Pass, target, source ast.Expr) bool {
	elem, ok := element(pass.TypesInfo.TypeOf(target))
	if !ok || !types.Identical(elem, types.Typ[types.Byte]) {
		return false
	}
	return byteSource(pass.TypesInfo.TypeOf(source))
}

// byteSource reports whether every type in t's type set is a string or a byte
// slice.
func byteSource(t types.Type) bool {
	terms, ok := typeTerms(t)
	if !ok || len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		switch u := term.(type) {
		case *types.Basic:
			if u.Info()&types.IsString == 0 {
				return false
			}
		case *types.Slice:
			if !types.Identical(u.Elem(), types.Typ[types.Byte]) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// element returns the element type of t, where t is a slice or a type
// parameter whose type set contains one slice element type.
func element(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}
	if slice, ok := t.Underlying().(*types.Slice); ok {
		return slice.Elem(), true
	}
	terms, ok := typeTerms(t)
	if !ok || len(terms) == 0 {
		return nil, false
	}
	var elem types.Type
	for _, term := range terms {
		slice, ok := term.(*types.Slice)
		if !ok || (elem != nil && !types.Identical(elem, slice.Elem())) {
			return nil, false
		}
		elem = slice.Elem()
	}
	return elem, elem != nil
}

// typeTerms returns the underlying types in t's type set, or reports false
// when the set is not bounded by type terms.
func typeTerms(t types.Type) ([]types.Type, bool) {
	if t == nil {
		return nil, false
	}
	if union, ok := t.(*types.Union); ok {
		var out []types.Type
		for term := range union.Terms() {
			out = append(out, term.Type().Underlying())
		}
		return out, len(out) > 0
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		return interfaceTerms(iface)
	}
	return []types.Type{t.Underlying()}, true
}

// interfaceTerms returns the underlying types in iface's type set, intersected
// across its embedded restrictions.
func interfaceTerms(iface *types.Interface) (result []types.Type, restricted bool) {
	for embed := range iface.EmbeddedTypes() {
		if unrestricted(embed) {
			continue
		}
		terms, ok := typeTerms(embed)
		if !ok || len(terms) == 0 {
			return nil, false
		}
		if !restricted {
			result, restricted = terms, true
			continue
		}
		result = intersect(result, terms)
		if len(result) == 0 {
			return nil, false
		}
	}
	return result, restricted
}

// intersect returns the terms present in both a and b.
func intersect(a, b []types.Type) (out []types.Type) {
	for _, x := range a {
		for _, y := range b {
			if types.Identical(x, y) {
				out = append(out, x)
				break
			}
		}
	}
	return out
}

// unrestricted reports whether t is an interface whose embedded types add no
// terms, as any and comparable are.
func unrestricted(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for embed := range iface.EmbeddedTypes() {
		if !unrestricted(embed) {
			return false
		}
	}
	return true
}

// report sends the diagnostic for a loop that copies one slice into another.
func report(pass *analysis.Pass, loop ast.Stmt) {
	pass.Report(analysis.Diagnostic{
		Pos:     loop.Pos(),
		End:     loop.End(),
		Message: "loop copies one element at a time",
	})
}
