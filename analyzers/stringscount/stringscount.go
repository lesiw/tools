// Package stringscount provides an analysis.Analyzer that reports a loop that
// counts the elements of a slice with a counter.
//
// The spec says a range over a slice produces iteration values from index 0 up
// to len(a)-1, and len returns the length of a slice. A loop that adds one to
// a counter on every pass leaves the counter at len(slice).
//
// The counter is an int, initialized to zero by the statement before the loop,
// and the body is one n++. The loop may range over the slice, or it may use a
// counted loop bounded by len(slice). A slice expression with a call other
// than a pure builtin or a conversion, a channel receive, or the address of a
// composite literal is left alone, and a counted loop leaves a bound with a
// *p expression alone. A loop over a map, string, array, or channel is left
// alone, as is a loop with any other statement in its body.
//
// The rule is adapted from the loop-to-call shape of staticcheck's S1011,
// which replaces a range-append loop with append. Here the call is len.
//
// The fix sets the counter to len(slice), and folds a zero declaration into
// that write where it can. In a range loop, the initialization stays when the
// slice expression contains the counter or a *p expression. When a comment
// lies between the initialization and the loop, the fix leaves the
// initialization in place. No fix is offered when a comment sits inside the
// loop, since the edit would delete it. The README catalogs every check,
// exemption, and fix.
package stringscount

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "stringscount",
	Doc:  "reports a loop that counts the elements of a slice with a counter",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				statements(pass, file, n.List)
			case *ast.CaseClause:
				statements(pass, file, n.Body)
			case *ast.CommClause:
				statements(pass, file, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// statements checks each loop in list against the statement before it.
func statements(pass *analysis.Pass, file *ast.File, list []ast.Stmt) {
	for i, stmt := range list {
		if i == 0 {
			continue
		}
		check(pass, file, list[i-1], stmt)
	}
}

// check reports stmt when it counts a slice in a counter initialized by prev.
func check(pass *analysis.Pass, file *ast.File, prev, stmt ast.Stmt) {
	var (
		counter *ast.Ident
		slice   ast.Expr
		ok      bool
	)
	switch loop := stmt.(type) {
	case *ast.RangeStmt:
		counter, slice, ok = ranged(pass, loop)
	case *ast.ForStmt:
		counter, slice, ok = counted(pass, loop)
	default:
		return
	}
	if !ok || !validCounter(pass, counter, stmt.Pos()) {
		return
	}
	foldable, ok := zeroed(pass, prev, counter)
	if !ok {
		return
	}
	call := "len(" + astexpr.Text(ast.Unparen(slice)) + ")"
	fix := fixes(pass, file, prev, stmt, counter, foldable, slice, call)
	pass.Report(analysis.Diagnostic{
		Pos:            stmt.Pos(),
		End:            stmt.End(),
		Message:        "loop can be replaced by " + call,
		SuggestedFixes: fix,
	})
}

// validCounter reports whether counter has type int and len is not shadowed
// at pos.
func validCounter(pass *analysis.Pass, counter *ast.Ident, pos token.Pos) bool {
	return isInt(pass.TypesInfo.TypeOf(counter)) && !shadowed(pass, pos)
}

// ranged returns the counter of a range loop that counts a slice, and reports
// false for any other loop.
func ranged(pass *analysis.Pass, loop *ast.RangeStmt) (*ast.Ident, ast.Expr, bool) {
	if !countableRange(pass, loop) {
		return nil, nil, false
	}
	counter, ok := increment(loop.Body)
	return counter, loop.X, ok
}

// countableRange reports whether loop ranges over a slice with no iteration
// value that the body can read.
func countableRange(pass *analysis.Pass, loop *ast.RangeStmt) bool {
	return blank(loop.Key) && blank(loop.Value) &&
		isSlice(pass, loop.X) && !astexpr.Effectful(pass, loop.X)
}

// counted returns the counter of a loop written
// for i := 0; i < len(slice); i++, and reports false for any other loop.
func counted(pass *analysis.Pass, loop *ast.ForStmt) (*ast.Ident, ast.Expr, bool) {
	counter, ok := increment(loop.Body)
	if !ok {
		return nil, nil, false
	}
	index, ok := countedInit(pass, loop.Init, counter)
	if !ok {
		return nil, nil, false
	}
	slice, ok := countedSlice(pass, loop.Cond, index, counter)
	if !ok || !countedPost(pass, loop.Post, index) {
		return nil, nil, false
	}
	return counter, slice, true
}

// countedInit returns the index of a counted loop's init when it declares the
// index at zero, and reports false for any other init.
func countedInit(pass *analysis.Pass, stmt ast.Stmt, counter *ast.Ident) (*ast.Ident, bool) {
	init, ok := stmt.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 {
		return nil, false
	}
	if len(init.Rhs) != 1 {
		return nil, false
	}
	index, ok := ast.Unparen(init.Lhs[0]).(*ast.Ident)
	if !ok || !zero(pass, init.Rhs[0]) {
		return nil, false
	}
	obj := pass.TypesInfo.ObjectOf(index)
	if obj == nil || obj == pass.TypesInfo.ObjectOf(counter) {
		return nil, false
	}
	return index, true
}

// countedSlice returns the slice of a counted loop's bound when the bound is
// len(slice), and reports false for any other condition.
func countedSlice(pass *analysis.Pass, cond ast.Expr, index, counter *ast.Ident) (ast.Expr, bool) {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.LSS || !astexpr.Same(pass, bin.X, index) {
		return nil, false
	}
	slice, ok := lenOperand(pass, bin.Y)
	if !ok || !isSlice(pass, slice) {
		return nil, false
	}
	if astexpr.Mentions(pass, slice, counter) {
		return nil, false
	}
	if astexpr.Mentions(pass, slice, index) {
		return nil, false
	}
	if astexpr.Effectful(pass, slice) || indirect(pass, slice) {
		return nil, false
	}
	return slice, true
}

// countedPost reports whether stmt increments index.
func countedPost(pass *analysis.Pass, stmt ast.Stmt, index *ast.Ident) bool {
	post, ok := stmt.(*ast.IncDecStmt)
	return ok && post.Tok == token.INC && astexpr.Same(pass, post.X, index)
}

// increment returns the counter of a body that only increments it, and
// reports false for any other body.
func increment(body *ast.BlockStmt) (*ast.Ident, bool) {
	if len(body.List) != 1 {
		return nil, false
	}
	inc, ok := body.List[0].(*ast.IncDecStmt)
	if !ok || inc.Tok != token.INC {
		return nil, false
	}
	counter, ok := ast.Unparen(inc.X).(*ast.Ident)
	return counter, ok && counter.Name != "_"
}

// zeroed reports whether prev sets counter to zero, and whether the whole
// initialization can be folded into the loop's replacement.
func zeroed(pass *analysis.Pass, prev ast.Stmt, counter *ast.Ident) (foldable, ok bool) {
	obj := pass.TypesInfo.ObjectOf(counter)
	if obj == nil {
		return false, false
	}
	switch prev := prev.(type) {
	case *ast.AssignStmt:
		return zeroAssign(pass, prev, obj)
	case *ast.DeclStmt:
		return zeroDecl(pass, prev, obj)
	}
	return false, false
}

// zeroAssign reports whether stmt sets obj to zero in an assignment or a
// short declaration.
func zeroAssign(pass *analysis.Pass, stmt *ast.AssignStmt, obj types.Object) (foldable, ok bool) {
	if stmt.Tok != token.DEFINE && stmt.Tok != token.ASSIGN {
		return false, false
	}
	if len(stmt.Lhs) != len(stmt.Rhs) {
		return false, false
	}
	found, alias := -1, false
	for i, lhs := range stmt.Lhs {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok {
			alias = true
			continue
		}
		if pass.TypesInfo.ObjectOf(id) != obj {
			continue
		}
		if found >= 0 {
			return false, false
		}
		found = i
	}
	if found < 0 || alias || !zero(pass, stmt.Rhs[found]) {
		return false, false
	}
	return stmt.Tok == token.DEFINE && len(stmt.Lhs) == 1, true
}

// zeroDecl reports whether stmt declares obj with a zero value.
func zeroDecl(pass *analysis.Pass, stmt *ast.DeclStmt, obj types.Object) (foldable, ok bool) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || decl.Tok != token.VAR {
		return false, false
	}
	for _, spec := range decl.Specs {
		spec, ok := spec.(*ast.ValueSpec)
		if !ok {
			return false, false
		}
		for i, name := range spec.Names {
			if pass.TypesInfo.ObjectOf(name) != obj {
				continue
			}
			if !zeroValue(pass, spec, i) {
				return false, false
			}
			return len(decl.Specs) == 1 && len(spec.Names) == 1, true
		}
	}
	return false, false
}

// zeroValue reports whether the value at index i in spec is the zero value.
func zeroValue(pass *analysis.Pass, spec *ast.ValueSpec, i int) bool {
	if len(spec.Values) == 0 {
		return true
	}
	if len(spec.Values) != len(spec.Names) {
		return false
	}
	return zero(pass, spec.Values[i])
}

// fixes returns the edit that replaces the loop, folding the initialization
// into it where the edit is safe and would not delete a comment.
func fixes(pass *analysis.Pass, file *ast.File, prev, loop ast.Stmt, counter *ast.Ident, foldable bool, slice ast.Expr, call string) []analysis.SuggestedFix {
	if commented(file, loop.Pos(), loop.End()) {
		return nil
	}
	if canFold(pass, file, prev, loop, counter, slice, foldable) {
		text := counter.Name + " := " + call
		return suggestion(text, prev.Pos(), loop.End())
	}
	return suggestion(counter.Name+" = "+call, loop.Pos(), loop.End())
}

// canFold reports whether the initialization can be replaced along with the
// loop.
func canFold(pass *analysis.Pass, file *ast.File, prev, loop ast.Stmt, counter *ast.Ident, slice ast.Expr, foldable bool) bool {
	if !foldable || commented(file, prev.Pos(), loop.End()) {
		return false
	}
	if astexpr.Mentions(pass, slice, counter) {
		return false
	}
	return !indirect(pass, slice)
}

// suggestion returns one fix that writes text over [lo, hi).
func suggestion(text string, lo, hi token.Pos) []analysis.SuggestedFix {
	return []analysis.SuggestedFix{{
		Message: "Replace with " + text,
		TextEdits: []analysis.TextEdit{{
			Pos:     lo,
			End:     hi,
			NewText: []byte(text),
		}},
	}}
}

// commented reports whether a comment lies in [lo, hi).
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// blank reports whether e is absent or the blank identifier.
func blank(e ast.Expr) bool {
	if e == nil {
		return true
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// zero reports whether e is the constant zero.
func zero(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	return ok && tv.Value != nil && constant.Sign(tv.Value) == 0
}

// isInt reports whether t is int.
func isInt(t types.Type) bool {
	return t != nil && types.Identical(t, types.Typ[types.Int])
}

// shadowed reports whether len resolves to something other than the builtin
// at pos.
func shadowed(pass *analysis.Pass, pos token.Pos) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent("len", pos)
	return obj != types.Universe.Lookup("len")
}

// isSlice reports whether e has a slice type.
func isSlice(pass *analysis.Pass, e ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Slice)
	return ok
}

// indirect reports whether e contains a pointer dereference.
func indirect(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		star, ok := n.(*ast.StarExpr)
		if !ok {
			return true
		}
		if tv, ok := pass.TypesInfo.Types[star]; !ok || !tv.IsType() {
			found = true
		}
		return !found
	})
	return found
}

// lenOperand returns the operand of e when e is len(operand).
func lenOperand(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("len") {
		return nil, false
	}
	return call.Args[0], true
}
