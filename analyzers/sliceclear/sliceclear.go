// Package sliceclear provides an analysis.Analyzer that reports a loop that
// zeroes every element of a slice.
//
// The spec says clear "sets all elements up to the length of s to the zero
// value of T".
//
// The rule is adapted from go-critic's sliceClear, which reports the counted
// loop; its documentation gives the range form as the replacement. x/tools has
// an open issue for a modernize check that replaces these loops with clear
// (go.dev/issue/82956).
//
// The fix rewrites the loop as clear(xs), and is withheld where the edit would
// delete a comment. The README catalogs every check, exemption, and fix.
package sliceclear

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"go/version"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "sliceclear",
	Doc:  "reports a loop that zeroes every element of a slice",
	Run:  run,
}

// loop is one loop that zeroes a slice.
type loop struct {
	pos, end token.Pos // the span of the loop
	body     *ast.BlockStmt
	index    *ast.Ident
	slice    ast.Expr
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// An empty version means the type checker enforced none, so clear is
		// in the language the file was checked against.
		v := pass.TypesInfo.FileVersions[file]
		if v != "" && version.Compare(v, "go1.21") < 0 {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			l, ok := loopOf(pass, n)
			if ok && !shadowed(pass, l.pos) {
				pass.Report(diagnose(file, l))
			}
			return true
		})
	}
	return nil, nil
}

// diagnose returns the diagnostic for one loop, with the rewrite as its fix
// where a comment in the loop would not be deleted with it.
func diagnose(file *ast.File, l loop) analysis.Diagnostic {
	var (
		// The slice is a chain of identifiers, selectors, indexes, literals,
		// and dereferences, which ExprString prints as written.
		call = "clear(" + types.ExprString(ast.Unparen(l.slice)) + ")"
		d    = analysis.Diagnostic{
			Pos:     l.pos,
			End:     l.end,
			Message: "loop can be replaced by " + call,
		}
	)
	if commented(file, l) {
		return d
	}
	d.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Replace with " + call,
		TextEdits: []analysis.TextEdit{{
			Pos:     l.pos,
			End:     l.end,
			NewText: []byte(call),
		}},
	}}
	return d
}

// commented reports whether a comment falls inside l, where the edit would
// delete it along with the loop.
func commented(file *ast.File, l loop) bool {
	for _, group := range file.Comments {
		if group.Pos() < l.end && group.End() > l.pos {
			return true
		}
	}
	return false
}

// shadowed reports whether clear resolves to something other than the builtin
// at pos.
func shadowed(pass *analysis.Pass, pos token.Pos) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent("clear", pos)
	return obj != types.Universe.Lookup("clear")
}

// loopOf returns n as a loop whose body zeroes a slice, and reports false for
// anything else.
func loopOf(pass *analysis.Pass, n ast.Node) (loop, bool) {
	var l loop
	switch n := n.(type) {
	case *ast.RangeStmt:
		l = loop{pos: n.Pos(), end: n.End(), body: n.Body}
		l.index, l.slice = ranged(pass, n)
	case *ast.ForStmt:
		l = loop{pos: n.Pos(), end: n.End(), body: n.Body}
		l.index, l.slice = counted(pass, n)
	default:
		return loop{}, false
	}
	if l.slice == nil || !zeroed(pass, l) {
		return loop{}, false
	}
	return l, true
}

// ranged returns the index and the slice of a range loop that declares its
// index, and nil for any other statement. A range over len(xs) is read as a
// range over xs.
func ranged(pass *analysis.Pass, stmt *ast.RangeStmt) (*ast.Ident, ast.Expr) {
	index, ok := stmt.Key.(*ast.Ident)
	if !ok || stmt.Tok != token.DEFINE {
		return nil, nil
	}
	if slice, ok := lenOperand(pass, stmt.X); ok {
		return index, slice
	}
	return index, stmt.X
}

// counted returns the index and the slice of a loop written
// for i := 0; i < len(xs); i++, and nil for any other statement.
func counted(pass *analysis.Pass, stmt *ast.ForStmt) (*ast.Ident, ast.Expr) {
	index, ok := atZero(pass, stmt.Init)
	if !ok {
		return nil, nil
	}
	obj := pass.TypesInfo.ObjectOf(index)
	cond, ok := stmt.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.LSS || !same(pass, cond.X, index) {
		return nil, nil
	}
	slice, ok := lenOperand(pass, cond.Y)
	if !ok || mentions(pass, slice, obj) {
		return nil, nil
	}
	post, ok := stmt.Post.(*ast.IncDecStmt)
	if !ok || post.Tok != token.INC || !same(pass, post.X, index) {
		return nil, nil
	}
	return index, slice
}

// atZero returns the index a counted loop declares at zero, and reports false
// for any other init statement.
func atZero(pass *analysis.Pass, stmt ast.Stmt) (*ast.Ident, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	index, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, false
	}
	obj := pass.TypesInfo.ObjectOf(index)
	if !isZero(pass, obj.Type(), assign.Rhs[0]) {
		return nil, false
	}
	return index, true
}

// mentions reports whether e reads the object obj.
func mentions(pass *analysis.Pass, e ast.Expr, obj types.Object) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if pass.TypesInfo.ObjectOf(ident) == obj {
				found = true
			}
		}
		return !found
	})
	return found
}

// lenOperand returns the operand of x when x is len(operand).
func lenOperand(pass *analysis.Pass, x ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(x).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return nil, false
	}
	return call.Args[0], true
}

// zeroed reports whether the loop's body assigns the element's zero value to
// the element at its index.
func zeroed(pass *analysis.Pass, l loop) bool {
	if len(l.body.List) != 1 {
		return false
	}
	assign, ok := l.body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	target, ok := ast.Unparen(assign.Lhs[0]).(*ast.IndexExpr)
	if !ok {
		return false
	}
	if !same(pass, target.X, l.slice) || !same(pass, target.Index, l.index) {
		return false
	}
	if t := pass.TypesInfo.TypeOf(target.X); t == nil || !sliceType(t) {
		return false
	}
	elem := pass.TypesInfo.TypeOf(target)
	return elem != nil && isZero(pass, elem, assign.Rhs[0])
}

// sliceType reports whether t is a slice, or a type parameter constrained by a
// union of slice types, where clear zeroes elements instead of deleting them.
func sliceType(t types.Type) bool {
	u := t.Underlying()
	if _, ok := u.(*types.Slice); ok {
		return true
	}
	iface, ok := u.(*types.Interface)
	if !ok || iface.NumEmbeddeds() != 1 {
		return false
	}
	union, ok := iface.EmbeddedType(0).(*types.Union)
	if !ok {
		return false
	}
	for term := range union.Terms() {
		if _, ok := term.Type().Underlying().(*types.Slice); !ok {
			return false
		}
	}
	return true
}

// isZero reports whether e is the zero value of type typ.
func isZero(pass *analysis.Pass, typ types.Type, e ast.Expr) bool {
	tv := pass.TypesInfo.Types[ast.Unparen(e)]
	if tv.IsNil() {
		return true
	}
	if tv.Value != nil {
		// An interface element boxes the constant, where clear writes nil. A
		// type parameter takes it as the zero value of every type in its set,
		// since the assignment checks the constant against them.
		var (
			_, param = typ.(*types.TypeParam)
			_, basic = typ.Underlying().(*types.Basic)
		)
		if !param && !basic {
			return false
		}
		return constZero(tv.Value)
	}
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 0 {
		return false
	}
	switch typ.Underlying().(type) {
	case *types.Struct, *types.Array:
		return true
	}
	return false
}

// constZero reports whether v is the zero constant of its kind.
func constZero(v constant.Value) bool {
	switch v.Kind() {
	case constant.Bool:
		return !constant.BoolVal(v)
	case constant.String:
		return constant.StringVal(v) == ""
	case constant.Int, constant.Float, constant.Complex:
		return constant.Sign(v) == 0
	}
	return false
}

// same reports whether a and b read the same value: the same identifiers,
// field names, indexes, literals, and dereferences. A call is left out, since
// two of them may differ.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	a, b = ast.Unparen(a), ast.Unparen(b)
	switch a := a.(type) {
	case *ast.Ident:
		ident, ok := b.(*ast.Ident)
		if !ok {
			return false
		}
		obj := pass.TypesInfo.ObjectOf(a)
		return obj != nil && obj == pass.TypesInfo.ObjectOf(ident)
	case *ast.SelectorExpr:
		sel, ok := b.(*ast.SelectorExpr)
		return ok && a.Sel.Name == sel.Sel.Name && same(pass, a.X, sel.X)
	case *ast.IndexExpr:
		index, ok := b.(*ast.IndexExpr)
		return ok && same(pass, a.X, index.X) &&
			same(pass, a.Index, index.Index)
	case *ast.StarExpr:
		star, ok := b.(*ast.StarExpr)
		return ok && same(pass, a.X, star.X)
	case *ast.BasicLit:
		lit, ok := b.(*ast.BasicLit)
		return ok && a.Kind == lit.Kind && a.Value == lit.Value
	}
	return false
}
