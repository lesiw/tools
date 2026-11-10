// Package afterhttperror provides an analysis.Analyzer that reports an
// http.Error call that the handler does not return after.
//
// http.Error writes the response and returns, so the handler continues and
// writes a second one. The client gets a status line followed by whatever the
// rest of the handler wrote.
package afterhttperror

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "afterhttperror",
	Doc:  "reports an http.Error call without a return after it",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				check(pass, n.List)
			case *ast.CaseClause:
				check(pass, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// check reports an http.Error call in list that a later statement follows.
func check(pass *analysis.Pass, list []ast.Stmt) {
	for i, stmt := range list {
		call := httpError(pass, stmt)
		if call == nil || i+1 == len(list) || returns(list[i+1]) {
			continue
		}
		// A statement that writes somewhere else, such as a log, does
		// not write the response a second time.
		if !writes(pass, list[i+1:], call.Args[0]) {
			continue
		}
		pass.Reportf(stmt.Pos(), "http.Error does not stop the handler")
	}
}

// writes reports whether any statement in list uses w, the writer that
// http.Error was given.
func writes(pass *analysis.Pass, list []ast.Stmt, w ast.Expr) bool {
	obj := pass.TypesInfo.ObjectOf(ident(w))
	if obj == nil {
		return true
	}
	var found bool
	for _, stmt := range list {
		ast.Inspect(stmt, func(n ast.Node) bool {
			if found {
				return false
			}
			id, ok := n.(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(id) == obj {
				found = true
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// ident returns the identifier e names, if it names one.
func ident(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}

// returns reports whether stmt leaves the handler.
func returns(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		// A panic leaves too, and so does a call to a helper that
		// ends the handler. Only a panic is certain.
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := call.Fun.(*ast.Ident)
		return ok && id.Name == "panic"
	}
	return false
}

// httpError returns the call if stmt is an http.Error call.
func httpError(pass *analysis.Pass, stmt ast.Stmt) *ast.CallExpr {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Error" {
		return nil
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "net/http" {
		return nil
	}
	return call
}
