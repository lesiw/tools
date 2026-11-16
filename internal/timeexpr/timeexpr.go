// Package timeexpr provides facts about time.Time expressions and rewrites
// that the analyzers share: the name a file uses for the time package, whether
// an expression is a time.Time or a *time.Time, whether a call is a time
// function, and whether a rewrite would join lines or drop a comment.
package timeexpr

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

// NowQualifier returns the name the file uses for the time package when e is a
// call to time.Now.
func NowQualifier(pass *analysis.Pass, e ast.Expr) (string, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return "", false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || !IsFunc(pass, sel.Sel, "Now") {
		return "", false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, true
}

// Text returns the text to write in place of e, and false when e is not a
// time.Time or a *time.Time. A *time.Time is written as the value it points
// at.
func Text(pass *analysis.Pass, e ast.Expr) (string, bool) {
	var (
		expr = ast.Unparen(e)
		text = astexpr.Text(expr)
		t    = pass.TypesInfo.TypeOf(e)
	)
	if !isValue(t) {
		if !IsPointer(t) {
			return "", false
		}
		unary, ok := expr.(*ast.UnaryExpr)
		if ok && unary.Op == token.AND {
			text = astexpr.Text(ast.Unparen(unary.X))
		} else {
			text = "*" + text
		}
	}
	return text, true
}

// IsFunc reports whether id is the function called name in package time.
func IsFunc(pass *analysis.Pass, id *ast.Ident, name string) bool {
	fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
	return ok && fn.Name() == name && isPkg(fn.Pkg())
}

// Fixable reports whether span lies on one line and contains no comment, so
// that a rewrite neither joins lines nor drops a comment. It reads written
// positions, which a //line directive does not move.
func Fixable(fset *token.FileSet, file *ast.File, span ast.Node) bool {
	var (
		start = fset.PositionFor(span.Pos(), false)
		end   = fset.PositionFor(span.End(), false)
	)
	if start.Line != end.Line {
		return false
	}
	for _, group := range file.Comments {
		if group.Pos() < span.End() && span.Pos() < group.End() {
			return false
		}
	}
	return true
}

// isValue reports whether t is time.Time.
func isValue(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Name() == "Time" && isPkg(named.Obj().Pkg())
}

// IsPointer reports whether t is a *time.Time.
func IsPointer(t types.Type) bool {
	ptr, ok := types.Unalias(t).(*types.Pointer)
	return ok && isValue(ptr.Elem())
}

// isPkg reports whether pkg is the time package.
func isPkg(pkg *types.Package) bool {
	return pkg != nil && pkg.Path() == "time"
}
