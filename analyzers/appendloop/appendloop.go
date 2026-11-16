// Package appendloop provides an analysis.Analyzer that reports a range loop
// that appends each element of a slice by index, one at a time.
//
// The section of the spec on appending to and copying slices specifies append,
// including append(dst, src...), which appends every element of src in one
// call. A loop that appends one element per pass may grow and copy the slice
// once per element. The body is dst = append(dst, src[i]) or v := src[i]
// followed by dst = append(dst, v). In the two-statement form the target
// cannot contain the value.
//
// The source and the target are usually parameters, so a caller can pass two
// slices over the same array. An append to the target can overwrite an element
// the source has not read yet. A source that is a different expression
// containing the target is left alone, since a bound such as cap(dst) can
// reach past the target's length. A loop over the target itself reads only
// elements before its captured length, so it carries a fix unless a comment
// inside the loop would be deleted. The spread append may grow once instead of
// once per element, so the final capacity can differ. The rule is otherwise
// report-only: the target and the source are usually parameters, and a caller
// can pass overlapping slices.
//
// The index forms are adapted from staticcheck's S1011. rangeappendall covers
// the value form.
package appendloop

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "appendloop",
	Doc:  "reports a range loop that appends each element of a slice by index",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if loop, ok := n.(*ast.RangeStmt); ok {
				check(pass, file, loop)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a loop that appends every element of its range expression to
// one slice.
func check(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt) {
	// A loop that assigns to a variable from the enclosing scope would lose
	// that write in the replacement.
	if loop.Tok != token.DEFINE {
		return
	}
	target, source, ok := match(pass, loop)
	if !ok || !accepts(pass, target, source) {
		return
	}
	if mentionsTarget(pass, target, source) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:            loop.Pos(),
		End:            loop.End(),
		Message:        "loop appends one element at a time",
		SuggestedFixes: fixes(pass, file, loop, target, source),
	})
}

// match returns the slice a loop appends to and the slice it ranges over.
func match(pass *analysis.Pass, loop *ast.RangeStmt) (target, src ast.Expr, ok bool) {
	if loop.Value != nil {
		value, ok := loop.Value.(*ast.Ident)
		if !ok || value.Name != "_" {
			return nil, nil, false
		}
	}
	key, ok := loop.Key.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	source := ast.Unparen(loop.X)
	switch len(loop.Body.List) {
	case 1:
		target, ok := appended(pass, loop.Body.List[0], func(e ast.Expr) bool {
			return indexes(pass, e, source, key)
		})
		return target, source, ok
	case 2:
		value, ok := indexed(pass, loop.Body.List[0], source, key)
		if !ok {
			return nil, nil, false
		}
		target, ok := appended(pass, loop.Body.List[1], func(e ast.Expr) bool {
			return astexpr.Same(pass, e, value)
		})
		if ok && astexpr.Mentions(pass, target, value) {
			return nil, nil, false
		}
		return target, source, ok
	}
	return nil, nil, false
}

// appended returns the slice that stmt appends an accepted value to, and
// reports false for any other statement.
func appended(pass *analysis.Pass, stmt ast.Stmt, accept func(ast.Expr) bool) (ast.Expr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || call.Ellipsis != token.NoPos {
		return nil, false
	}
	if !builtin(pass, call.Fun, "append") {
		return nil, false
	}
	target, first := ast.Unparen(assign.Lhs[0]), ast.Unparen(call.Args[0])
	if !variable(target) || !astexpr.Same(pass, target, first) {
		return nil, false
	}
	if !accept(ast.Unparen(call.Args[1])) {
		return nil, false
	}
	return target, true
}

// indexed returns the variable that stmt assigns from source[key].
func indexed(pass *analysis.Pass, stmt ast.Stmt, source ast.Expr, key *ast.Ident) (*ast.Ident, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	value, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || value.Name == "_" {
		return nil, false
	}
	if !indexes(pass, ast.Unparen(assign.Rhs[0]), source, key) {
		return nil, false
	}
	return value, true
}

// indexes reports whether e reads source at key.
func indexes(pass *analysis.Pass, e, source ast.Expr, key *ast.Ident) bool {
	index, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok {
		return false
	}
	return sameSource(pass, index.X, source) &&
		astexpr.Same(pass, index.Index, key)
}

// sameSource reports whether a and b read the same slice expression. It adds
// slice expressions to the forms astexpr.Same compares.
func sameSource(pass *analysis.Pass, a, b ast.Expr) bool {
	a, b = ast.Unparen(a), ast.Unparen(b)
	if astexpr.Same(pass, a, b) {
		return true
	}
	as, ok := a.(*ast.SliceExpr)
	if !ok {
		return false
	}
	bs, ok := b.(*ast.SliceExpr)
	if !ok {
		return false
	}
	return as.Slice3 == bs.Slice3 && sameOptional(pass, as.Low, bs.Low) &&
		sameOptional(pass, as.High, bs.High) &&
		sameOptional(pass, as.Max, bs.Max) &&
		sameSource(pass, as.X, bs.X)
}

// sameOptional reports whether a and b are both absent or read the same
// expression.
func sameOptional(pass *analysis.Pass, a, b ast.Expr) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameSource(pass, a, b)
}

// builtin reports whether e is an identifier for the predeclared name.
func builtin(pass *analysis.Pass, e ast.Expr, name string) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup(name)
}

// accepts reports whether append(target, source...) type-checks where
// append(target, value) did. The string special case is not included.
func accepts(pass *analysis.Pass, target, source ast.Expr) bool {
	elem, ok := element(pass.TypesInfo.TypeOf(target))
	if !ok {
		return false
	}
	from := pass.TypesInfo.TypeOf(source)
	return from != nil && types.AssignableTo(from, types.NewSlice(elem))
}

// element returns the element type of t, where t is a slice or a type
// parameter whose type set contains one slice type.
func element(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return u.Elem(), true
	case *types.Interface:
		return constrained(u)
	}
	return nil, false
}

// constrained returns the element type of a type set that contains one slice
// type.
func constrained(iface *types.Interface) (types.Type, bool) {
	if iface.NumEmbeddeds() != 1 {
		return nil, false
	}
	embed := iface.EmbeddedType(0)
	if union, ok := embed.(*types.Union); ok {
		return terms(union)
	}
	slice, ok := embed.Underlying().(*types.Slice)
	if !ok {
		return nil, false
	}
	return slice.Elem(), true
}

// terms returns the element type shared by every term of union.
func terms(union *types.Union) (types.Type, bool) {
	var elem types.Type
	for term := range union.Terms() {
		slice, ok := term.Type().Underlying().(*types.Slice)
		if !ok || (elem != nil && !types.Identical(elem, slice.Elem())) {
			return nil, false
		}
		elem = slice.Elem()
	}
	return elem, elem != nil
}

// mentionsTarget reports whether source is a different expression that
// mentions target.
func mentionsTarget(pass *analysis.Pass, target, source ast.Expr) bool {
	return !astexpr.Same(pass, target, source) &&
		astexpr.Mentions(pass, source, target)
}

// variable reports whether e is a name, a field path, or a dereference, which
// evaluates to the same value each time.
func variable(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return variable(e.X)
	case *ast.StarExpr:
		return variable(e.X)
	}
	return false
}

// fixes returns the edit that replaces a loop over the target itself with the
// spread append.
func fixes(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt, target, source ast.Expr) []analysis.SuggestedFix {
	if !astexpr.Same(pass, target, source) || commented(file, loop) {
		return nil
	}
	var (
		dst    = astexpr.Text(target)
		spread = "append(" + dst + ", " + dst + "...)"
	)
	return []analysis.SuggestedFix{{
		Message: "Replace with " + dst + " = " + spread,
		TextEdits: []analysis.TextEdit{{
			Pos:     loop.Pos(),
			End:     loop.End(),
			NewText: []byte(dst + " = " + spread),
		}},
	}}
}

// commented reports whether a comment lies inside loop.
func commented(file *ast.File, loop *ast.RangeStmt) bool {
	for _, group := range file.Comments {
		if group.Pos() > loop.Pos() && group.End() < loop.End() {
			return true
		}
	}
	return false
}
