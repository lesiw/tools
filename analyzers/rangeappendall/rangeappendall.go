// Package rangeappendall provides an analysis.Analyzer that reports a range
// loop that appends each element of a slice one at a time.
//
// The section of the spec on appending to and copying slices defines
// append(dst, src...), which appends every element of src in one call. A loop
// that appends one element per pass writes the same values when the two slices
// do not share an array, and may grow and copy the slice once per element.
//
// Report only: the target and the source are usually parameters, so a caller
// can pass two views of one array, and where the source reaches past the
// target's length the loop reads elements that earlier passes overwrote. A
// loop that ranges over the target cannot read what it writes, and that loop
// carries a fix.
//
// The rule is inspired by go-critic's rangeAppendAll, which reports the
// opposite loop. The README catalogs every check and exemption.
package rangeappendall

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "rangeappendall",
	Doc:  "reports a range loop that appends each element of a slice one at a time",
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

// check reports a loop whose whole body appends the range's value to a slice.
func check(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt) {
	// A loop that assigns to a variable from the enclosing scope leaves that
	// variable holding the last element.
	if loop.Tok != token.DEFINE {
		return
	}
	value, ok := loop.Value.(*ast.Ident)
	if !ok {
		return
	}
	if len(loop.Body.List) != 1 {
		return
	}
	target, ok := appended(pass, loop.Body.List[0], value)
	if !ok {
		return
	}
	source := ast.Unparen(loop.X)
	if !accepts(pass, target, source) {
		return
	}
	dst := astexpr.Text(target)
	src := astexpr.Text(source)
	// A loop over the target reads only the elements before the target's
	// length, which the appends do not write. Any other expression that names
	// the target can reach past the length.
	if dst != src && mentions(pass, source, root(pass, target)) {
		return
	}
	spread := "append(" + dst + ", " + src + "...)"
	pass.Report(analysis.Diagnostic{
		Pos:            loop.Pos(),
		End:            loop.End(),
		Message:        "loop can be replaced by " + spread,
		SuggestedFixes: fixes(file, loop, dst, spread),
	})
}

// fixes returns the edit that replaces loop, for the one loop whose range
// expression is its own target. The edit is withheld where it would delete a
// comment.
func fixes(file *ast.File, loop *ast.RangeStmt, dst, spread string) []analysis.SuggestedFix {
	if dst != astexpr.Text(ast.Unparen(loop.X)) || commented(file, loop) {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message: "Replace with " + dst + " = " + spread,
		TextEdits: []analysis.TextEdit{{
			Pos:     loop.Pos(),
			End:     loop.End(),
			NewText: []byte(dst + " = " + spread),
		}},
	}}
}

// appended returns the slice that stmt appends value to, and reports false for
// any other statement.
func appended(pass *analysis.Pass, stmt ast.Stmt, value *ast.Ident) (ast.Expr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || !builtin(pass, call.Fun, "append") {
		return nil, false
	}
	target, first := ast.Unparen(assign.Lhs[0]), ast.Unparen(call.Args[0])
	if !variable(target) || astexpr.Text(target) != astexpr.Text(first) {
		return nil, false
	}
	arg, ok := ast.Unparen(call.Args[1]).(*ast.Ident)
	if !ok || pass.TypesInfo.ObjectOf(arg) != pass.TypesInfo.ObjectOf(value) {
		return nil, false
	}
	return target, true
}

// builtin reports whether e names the predeclared name.
func builtin(pass *analysis.Pass, e ast.Expr, name string) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup(name)
}

// accepts reports whether append(target, src...) type-checks where
// append(target, value) did.
func accepts(pass *analysis.Pass, target, src ast.Expr) bool {
	elem, ok := element(pass.TypesInfo.TypeOf(target))
	if !ok {
		return false
	}
	from := pass.TypesInfo.TypeOf(src)
	return from != nil && types.AssignableTo(from, types.NewSlice(elem))
}

// element returns the element type of t, where t is a slice or a type
// parameter whose type set holds one slice type.
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

// constrained returns the element type of a type set holding one slice type.
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

// terms returns the element type that every term of union holds.
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

// variable reports whether e is a name or a field path, which evaluates to the
// same value each time.
func variable(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return variable(e.X)
	}
	return false
}

// root returns the object that the leftmost name in e refers to.
func root(pass *analysis.Pass, e ast.Expr) types.Object {
	switch e := e.(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(e)
	case *ast.SelectorExpr:
		return root(pass, e.X)
	}
	return nil
}

// mentions reports whether e names obj.
func mentions(pass *analysis.Pass, e ast.Expr, obj types.Object) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && pass.TypesInfo.ObjectOf(id) == obj {
			found = true
		}
		return !found
	})
	return found
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
