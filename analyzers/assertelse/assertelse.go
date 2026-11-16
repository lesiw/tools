// Package assertelse provides an analysis.Analyzer that reports a value read
// in the branch that runs when a type assertion fails.
//
// A two-value type assertion yields the zero value of the asserted type when
// it fails (https://go.dev/ref/spec#Type_assertions). An if statement is an
// implicit block (https://go.dev/ref/spec#Blocks), and a variable declared in
// its init is in scope to the end of that block
// (https://go.dev/ref/spec#Declarations_and_scope), so a read in the failing
// branch gets that zero value.
//
//	if v, ok := x.(int); ok {
//		print(v)
//	} else {
//		print(v)
//	}
//
// The README catalogs the reported forms and the reads that are left alone.
//
// Report only: reading the asserted expression and handling the failure are
// both plausible repairs.
//
// The rule is adapted from staticcheck's SA9008.
package assertelse

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "assertelse",
	Doc:      "reports a value read in the branch where a type assertion fails",
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

// assertions maps the opening parenthesis of a comma-ok type assertion to the
// instruction.
type assertions map[token.Pos]*ssa.TypeAssert

// binding is the value and success result an assertion binds.
type binding struct {
	assert *ast.TypeAssertExpr
	value  *ast.Ident
	result *ast.Ident
}

// span is a half-open source range.
type span struct {
	start token.Pos
	end   token.Pos
}

// site locates the branch being checked and its first read.
type site struct {
	branch ast.Node
	head   token.Pos
	read   token.Pos
	skip   []span
}

func run(pass *analysis.Pass) (any, error) {
	result := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range result.SrcFuncs {
		check(pass, fn)
	}
	return nil, nil
}

// checker holds the state one function shares across its if statements.
type checker struct {
	pass   *analysis.Pass
	found  assertions
	syntax ast.Node
}

// check reports the if statements in fn whose failing branch reads the value
// the assertion bound.
func check(pass *analysis.Pass, fn *ssa.Function) {
	node := fn.Syntax()
	if node == nil {
		return
	}
	c := checker{pass, commaOk(fn), node}
	ast.Inspect(node, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.IfStmt); ok {
			c.checkIf(stmt)
		}
		return true
	})
}

// commaOk returns the comma-ok type assertions in fn, keyed by the position of
// their opening parenthesis.
func commaOk(fn *ssa.Function) assertions {
	out := make(assertions)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if assert, ok := instr.(*ssa.TypeAssert); ok && assert.CommaOk {
				out[assert.Pos()] = assert
			}
		}
	}
	return out
}

// checkIf reports stmt when its init makes a two-value type assertion and the
// branch that runs on failure reads the bound value.
func (c checker) checkIf(stmt *ast.IfStmt) {
	b, ok := twoValue(stmt)
	if !ok {
		return
	}
	branch := failing(c.pass, stmt, b.result)
	if branch == nil {
		return
	}
	obj := c.pass.TypesInfo.ObjectOf(b.value)
	if addressed(c.pass, c.syntax, obj) {
		return
	}
	skip := ignoredSpans(c.pass, branch)
	read := firstRead(c.pass, branch, obj, skip)
	if read == nil {
		return
	}
	assertion := c.found[b.assert.Lparen]
	if assertion == nil {
		return
	}
	at := site{
		branch: branch,
		head:   stmt.Body.Pos(),
		read:   read.Pos(),
		skip:   skip,
	}
	if !readsIn(assertion, at) {
		if captured(assertion) {
			return
		}
		if !copied(branch, read) && !ranged(branch, read) {
			return
		}
		if assignedBefore(c.pass, branch, obj, read.Pos()) {
			return
		}
	}
	c.pass.Reportf(
		read.Pos(),
		"%s is the zero value in the branch where the type assertion fails",
		read.Name,
	)
}

// twoValue returns the type assertion and the names it binds from stmt's init
// statement.
func twoValue(stmt *ast.IfStmt) (binding, bool) {
	assign, ok := stmt.Init.(*ast.AssignStmt)
	if !ok || (assign.Tok != token.DEFINE && assign.Tok != token.ASSIGN) {
		return binding{}, false
	}
	if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return binding{}, false
	}
	assert, ok := ast.Unparen(assign.Rhs[0]).(*ast.TypeAssertExpr)
	if !ok || assert.Type == nil {
		return binding{}, false
	}
	value, ok := ast.Unparen(assign.Lhs[0]).(*ast.Ident)
	if !ok || value.Name == "_" {
		return binding{}, false
	}
	result, ok := ast.Unparen(assign.Lhs[1]).(*ast.Ident)
	if !ok || result.Name == "_" {
		return binding{}, false
	}
	return binding{assert, value, result}, true
}

// failing returns the branch stmt runs when the assertion fails, or nil when
// the condition tests something else or there is no such branch.
func failing(pass *analysis.Pass, stmt *ast.IfStmt, result *ast.Ident) ast.Node {
	object := pass.TypesInfo.ObjectOf(result)
	if id, ok := ast.Unparen(stmt.Cond).(*ast.Ident); ok {
		if pass.TypesInfo.ObjectOf(id) == object {
			return stmt.Else
		}
		return nil
	}
	not, ok := ast.Unparen(stmt.Cond).(*ast.UnaryExpr)
	if !ok || not.Op != token.NOT {
		return nil
	}
	id, ok := ast.Unparen(not.X).(*ast.Ident)
	if !ok || pass.TypesInfo.ObjectOf(id) != object {
		return nil
	}
	return stmt.Body
}

// readsIn reports whether a use of the type assertion's value lies in branch.
// The uses of a conversion are followed, since the result is computed from the
// value. A store outside branch into a local that holds the assertion value is
// followed through the local's reads, since the compiler makes that local when
// a field or element of the value is read. Any other use is read where it
// appears, and a phi is not followed at all.
func readsIn(assert *ssa.TypeAssert, at site) bool {
	var (
		seen  = make(map[ssa.Value]struct{})
		found bool
		visit func(ssa.Value)
	)
	visit = func(value ssa.Value) {
		if found {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		refs := value.Referrers()
		if refs == nil {
			return
		}
		for _, ref := range *refs {
			if found {
				return
			}
			switch ref := ref.(type) {
			case *ssa.Extract, *ssa.Phi:
			case *ssa.ChangeInterface, *ssa.ChangeType, *ssa.Convert,
				*ssa.MakeInterface, *ssa.MultiConvert,
				*ssa.SliceToArrayPointer:
				visit(ref.(ssa.Value))
			case *ssa.Store:
				if within(at.branch, ref.Pos()) {
					found = true
					continue
				}
				alloc, ok := ref.Addr.(*ssa.Alloc)
				if !ok || !ref.Pos().IsValid() || ref.Pos() >= at.head {
					continue
				}
				if allocRead(alloc, ref, at) {
					found = true
				}
			default:
				if at.counts(ref.Pos()) {
					found = true
				}
			}
		}
	}
	refs := assert.Referrers()
	if refs == nil {
		return false
	}
	for _, ref := range *refs {
		if extract, ok := ref.(*ssa.Extract); ok && extract.Index == 0 {
			visit(extract)
		}
	}
	return found
}

// allocRead reports whether alloc, a local the init store fills from the
// assertion value, is read in at.branch before anything writes it.
func allocRead(alloc *ssa.Alloc, init *ssa.Store, at site) bool {
	refs := alloc.Referrers()
	if refs == nil || escapes(alloc) {
		return false
	}
	for _, ref := range *refs {
		switch ref := ref.(type) {
		case *ssa.Store:
			if ref == init {
				continue
			}
			if within(at.branch, ref.Pos()) && ref.Pos() <= at.read {
				return false
			}
		case *ssa.UnOp, *ssa.Field, *ssa.Index, *ssa.Lookup, *ssa.Range,
			*ssa.Next, *ssa.FieldAddr, *ssa.IndexAddr, *ssa.Slice,
			*ssa.DebugRef:
		default:
			return false
		}
	}
	if written(alloc, at) {
		return false
	}
	for _, ref := range *refs {
		switch ref := ref.(type) {
		case *ssa.Store, *ssa.DebugRef:
			continue
		case *ssa.FieldAddr, *ssa.IndexAddr, *ssa.Slice:
			if readAddr(ref.(ssa.Value), at) {
				return true
			}
			continue
		}
		if at.counts(ref.Pos()) {
			return true
		}
	}
	return false
}

// readAddr reports whether addr is read in at.branch. A store through addr
// writes the value, and is not a read.
func readAddr(addr ssa.Value, at site) bool {
	refs := addr.Referrers()
	if refs == nil {
		return false
	}
	for _, ref := range *refs {
		if store, ok := ref.(*ssa.Store); ok && store.Addr == addr {
			continue
		}
		if at.counts(ref.Pos()) {
			return true
		}
	}
	return false
}

// escapes reports whether the address of value leaves the function or is
// stored where another function can write through it.
func escapes(value ssa.Value) bool {
	var (
		seen  = make(map[ssa.Value]struct{})
		found bool
		walk  func(ssa.Value)
	)
	walk = func(value ssa.Value) {
		if found {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		refs := value.Referrers()
		if refs == nil {
			return
		}
		for _, ref := range *refs {
			switch ref := ref.(type) {
			case *ssa.UnOp, *ssa.Field, *ssa.Index, *ssa.Lookup,
				*ssa.Range, *ssa.Next, *ssa.DebugRef:
			case *ssa.Store:
				if ref.Val == value {
					found = true
					return
				}
			case *ssa.FieldAddr:
				walk(ref)
			case *ssa.IndexAddr:
				walk(ref)
			case *ssa.Slice:
				walk(ref)
			case *ssa.ChangeType, *ssa.Convert, *ssa.MultiConvert,
				*ssa.SliceToArrayPointer:
				walk(ref.(ssa.Value))
			default:
				found = true
				return
			}
		}
	}
	walk(value)
	return found
}

// written reports whether a store through alloc or one of its fields or
// elements comes before at.read in at.branch, since that store can replace the
// zero value.
func written(alloc *ssa.Alloc, at site) bool {
	var (
		seen  = make(map[ssa.Value]struct{})
		found bool
		walk  func(ssa.Value)
	)
	walk = func(value ssa.Value) {
		if found {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		refs := value.Referrers()
		if refs == nil {
			return
		}
		for _, ref := range *refs {
			switch ref := ref.(type) {
			case *ssa.Store:
				if within(at.branch, ref.Pos()) && ref.Pos() <= at.read {
					found = true
					return
				}
			case *ssa.FieldAddr:
				walk(ref)
			case *ssa.IndexAddr:
				walk(ref)
			case *ssa.Slice:
				walk(ref)
			case *ssa.ChangeType, *ssa.Convert, *ssa.MultiConvert,
				*ssa.SliceToArrayPointer:
				walk(ref.(ssa.Value))
			}
		}
	}
	walk(alloc)
	return found
}

// counts reports whether pos is a use in at.branch that is not discarded.
func (at site) counts(pos token.Pos) bool {
	if !within(at.branch, pos) {
		return false
	}
	return !skipped(at.skip, pos)
}

// within reports whether pos lies inside branch.
func within(branch ast.Node, pos token.Pos) bool {
	return pos.IsValid() && pos >= branch.Pos() && pos < branch.End()
}

// addressed reports whether the function takes the address of obj, or of a
// field or element of obj.
func addressed(pass *analysis.Pass, node ast.Node, obj types.Object) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		unary, ok := n.(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			return true
		}
		id, ok := ast.Unparen(base(unary.X)).(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(id) == obj {
			found = true
			return false
		}
		return true
	})
	return found
}

// base returns the expression a selector, index or slice starts from.
func base(e ast.Expr) ast.Expr {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		default:
			return e
		}
	}
}

// firstRead returns the first identifier in branch that reads obj. An
// assignment target and a range variable are not reads.
func firstRead(pass *analysis.Pass, branch ast.Node, obj types.Object, skip []span) *ast.Ident {
	var (
		found *ast.Ident
		stack []ast.Node
	)
	ast.Inspect(branch, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if id, ok := n.(*ast.Ident); ok {
			match := pass.TypesInfo.ObjectOf(id) == obj
			if match && !target(stack, id) && !skipped(skip, id.Pos()) {
				found = id
				return false
			}
		}
		stack = append(stack, n)
		return true
	})
	return found
}

// ranged reports whether read is the expression a range statement ranges
// over. A range over the value reads it even when the body never runs.
func ranged(branch ast.Node, read *ast.Ident) bool {
	path := enclosing(branch, read)
	for _, n := range path {
		stmt, ok := n.(*ast.RangeStmt)
		if !ok {
			continue
		}
		return read.Pos() >= stmt.X.Pos() && read.End() <= stmt.X.End()
	}
	return false
}

// enclosing returns the nodes above read in branch.
func enclosing(branch ast.Node, read *ast.Ident) (found []ast.Node) {
	var stack []ast.Node
	ast.Inspect(branch, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if n == ast.Node(read) {
			found = append(found, stack...)
			return false
		}
		stack = append(stack, n)
		return true
	})
	return found
}

// captured reports whether the assertion value is stored in a local whose
// address leaves the function, where another function can write it.
func captured(assert *ssa.TypeAssert) bool {
	refs := assert.Referrers()
	if refs == nil {
		return false
	}
	for _, ref := range *refs {
		extract, ok := ref.(*ssa.Extract)
		if !ok || extract.Index != 0 {
			continue
		}
		uses := extract.Referrers()
		if uses == nil {
			continue
		}
		for _, use := range *uses {
			store, ok := use.(*ssa.Store)
			if !ok {
				continue
			}
			alloc, ok := store.Addr.(*ssa.Alloc)
			if ok && escapes(alloc) {
				return true
			}
		}
	}
	return false
}

// copied reports whether read is the right side of an assignment. A local that
// receives the value this way is merged with the values from other paths, so
// the read leaves no instruction of its own.
func copied(branch ast.Node, read *ast.Ident) bool {
	path := enclosing(branch, read)
	for _, n := range path {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		stmt, ok := n.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for _, rhs := range stmt.Rhs {
			if read.Pos() >= rhs.Pos() && read.End() <= rhs.End() {
				return true
			}
		}
		return false
	}
	return false
}

// assignedBefore reports whether an assignment to obj comes before pos in
// branch.
func assignedBefore(pass *analysis.Pass, branch ast.Node, obj types.Object, pos token.Pos) bool {
	var found bool
	ast.Inspect(branch, func(n ast.Node) bool {
		if found {
			return false
		}
		var targets []ast.Expr
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			targets = stmt.Lhs
		case *ast.RangeStmt:
			targets = []ast.Expr{stmt.Key, stmt.Value}
		case *ast.IncDecStmt:
			targets = []ast.Expr{stmt.X}
		}
		for _, target := range targets {
			if target == nil || target.Pos() >= pos {
				continue
			}
			id, ok := ast.Unparen(base(target)).(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(id) == obj {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// ignoredSpans returns the source ranges whose reads have no effect: an
// identifier assigned to a blank identifier and an argument to unsafe.Sizeof,
// unsafe.Alignof, or unsafe.Offsetof.
func ignoredSpans(pass *analysis.Pass, branch ast.Node) (out []span) {
	ast.Inspect(branch, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return true
			}
			if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
				return true
			}
			for i, rhs := range n.Rhs {
				if !blank(n.Lhs[i]) {
					continue
				}
				if _, ok := ast.Unparen(rhs).(*ast.Ident); ok {
					out = append(out, span{rhs.Pos(), rhs.End()})
				}
			}
		case *ast.CallExpr:
			if unsafeCall(pass, n) {
				for _, arg := range n.Args {
					out = append(out, span{arg.Pos(), arg.End()})
				}
			}
		}
		return true
	})
	return out
}

// unsafeCall reports whether call is unsafe.Sizeof, unsafe.Alignof, or
// unsafe.Offsetof.
func unsafeCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fun := pass.TypesInfo.Uses[sel.Sel]
	if fun == nil || fun.Pkg() == nil || fun.Pkg().Path() != "unsafe" {
		return false
	}
	switch fun.Name() {
	case "Sizeof", "Alignof", "Offsetof":
		return true
	}
	return false
}

// blank reports whether e is the blank identifier.
func blank(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// skipped reports whether pos lies in one of spans.
func skipped(spans []span, pos token.Pos) bool {
	for _, s := range spans {
		if pos >= s.start && pos < s.end {
			return true
		}
	}
	return false
}

// target reports whether id is an assignment target or part of one. A
// compound assignment and an increment read the old value.
func target(stack []ast.Node, id *ast.Ident) bool {
	for _, n := range slices.Backward(stack) {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			if stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE {
				return false
			}
			return targetIn(stmt.Lhs, id)
		case *ast.RangeStmt:
			return targetIn([]ast.Expr{stmt.Key, stmt.Value}, id)
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		}
	}
	return false
}

// targetIn reports whether id is the base of one of exprs.
func targetIn(exprs []ast.Expr, id *ast.Ident) bool {
	for _, expr := range exprs {
		if expr != nil && base(expr) == ast.Expr(id) {
			return true
		}
	}
	return false
}
