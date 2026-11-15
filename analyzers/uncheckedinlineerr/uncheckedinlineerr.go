// Package uncheckedinlineerr provides an analysis.Analyzer that reports an
// empty body in an if that checks an error from its init.
//
//	func write(w io.Writer, src string) error {
//		if _, err := io.WriteString(w, src); err != nil {
//		}
//		return nil
//	}
//
// Go's Code Review Comments say to check an error, then "handle the error,
// return it, or, in truly exceptional situations, panic".
//
// The idea comes from go-critic's uncheckedInlineErr, which reports an if
// whose condition tests a different error than its init assigns, and from
// staticcheck's SA9003, which reports an empty if or else body whatever the
// condition.
//
// The README catalogs the check and its exemptions.
//
// A fix would have to write the handling, which the tool cannot invent, so the
// rule reports without one.
package uncheckedinlineerr

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "uncheckedinlineerr",
	Doc:  "reports an empty body in an if that checks an error from its init",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if ifStmt, ok := n.(*ast.IfStmt); ok {
				check(pass, file, ifStmt)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each error the init assigns, when the condition mentions it
// and neither branch holds a statement or a comment.
func check(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt) {
	if len(ifStmt.Body.List) > 0 || commented(pass, file, ifStmt.Body) {
		return
	}
	switch e := ifStmt.Else.(type) {
	case nil:
	case *ast.BlockStmt:
		if len(e.List) > 0 || commented(pass, file, e) {
			return
		}
	default:
		// An else if holds an if statement, so it is not empty.
		return
	}
	for _, id := range assigned(pass, ifStmt.Init) {
		obj := pass.TypesInfo.ObjectOf(id)
		if !mentions(pass, ifStmt.Cond, obj) {
			continue
		}
		pass.Reportf(id.Pos(), "%s is checked in an if with an empty body", id.Name)
	}
}

// assigned returns the identifiers of the errors that init assigns.
func assigned(pass *analysis.Pass, init ast.Stmt) (ids []*ast.Ident) {
	assign, ok := init.(*ast.AssignStmt)
	if !ok {
		return nil
	}
	for i, lhs := range assign.Lhs {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok || converts(pass, rhs(assign, i)) {
			continue
		}
		obj := pass.TypesInfo.ObjectOf(id)
		// A blank identifier has no object.
		if obj == nil {
			continue
		}
		if isError(obj.Type()) {
			ids = append(ids, id)
		}
	}
	return ids
}

// rhs returns the expression that produces the value assigned to Lhs[i]. A
// call returning several values stands once on the right and is the value for
// every variable it feeds.
func rhs(assign *ast.AssignStmt, i int) ast.Expr {
	if len(assign.Rhs) == 1 {
		return assign.Rhs[0]
	}
	return assign.Rhs[i]
}

// converts reports whether e converts rather than calls, since a conversion
// cannot fail.
func converts(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	return ok && pass.TypesInfo.Types[call.Fun].IsType()
}

// isError reports whether t implements error. Implements reads a type
// parameter's method set, so a type term such as [P *net.AddrError] adds no
// methods.
func isError(t types.Type) bool {
	var (
		errType = types.Universe.Lookup("error").Type()
		err, ok = errType.Underlying().(*types.Interface)
	)
	return ok && types.Implements(t, err)
}

// mentions reports whether cond names obj.
func mentions(pass *analysis.Pass, cond ast.Expr, obj types.Object) bool {
	var found bool
	ast.Inspect(cond, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && pass.TypesInfo.ObjectOf(id) == obj {
			found = true
		}
		return !found
	})
	return found
}

// commented reports whether b holds a comment on a line of its own.
func commented(pass *analysis.Pass, file *ast.File, b *ast.BlockStmt) bool {
	open := pass.Fset.Position(b.Lbrace).Line
	for _, group := range file.Comments {
		// A comment that ends at or after the closing brace follows the
		// body.
		if group.Pos() < b.Lbrace || group.End() >= b.Rbrace {
			continue
		}
		if pass.Fset.Position(group.Pos()).Line > open {
			return true
		}
	}
	return false
}
