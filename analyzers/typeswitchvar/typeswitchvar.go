// Package typeswitchvar provides an analysis.Analyzer that reports a type
// switch whose cases assert the value again.
//
// switch v := x.(type) binds v to the case's type, so an assertion inside the
// case repeats work the switch already did. Like go-critic's typeSwitchVar.
package typeswitchvar

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "typeswitchvar",
	Doc:  "reports a type switch whose cases assert the value again",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.TypeSwitchStmt)
			if !ok || asserts(stmt.Assign) {
				return true
			}
			if !assertsAgain(pass, stmt) {
				return true
			}
			pass.Reportf(stmt.Switch, "type switch binds nothing; each case asserts again")
			return true
		})
	}
	return nil, nil
}

// asserts reports whether the switch's assign is already a binding.
func asserts(stmt ast.Stmt) bool {
	_, ok := stmt.(*ast.AssignStmt)
	return ok
}

// assertsAgain reports whether a case re-asserts the switched value.
func assertsAgain(pass *analysis.Pass, stmt *ast.TypeSwitchStmt) bool {
	expr, ok := stmt.Assign.(*ast.ExprStmt)
	if !ok {
		return false
	}
	assert, ok := expr.X.(*ast.TypeAssertExpr)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(ident(assert.X))
	if obj == nil {
		return false
	}
	// A binding only removes the assertion in a case that names exactly
	// one type: a multi-type case binds an interface, and default binds
	// the original value.
	var eligible int
	for _, stmt := range stmt.Body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		if len(clause.List) != 1 {
			return false // a binding would not remove every assertion
		}
		typ := pass.TypesInfo.TypeOf(clause.List[0])
		if typ == nil {
			continue
		}
		eligible++
		if !reasserts(pass, clause.Body, obj, typ) {
			return false
		}
	}
	return eligible > 0
}

// reasserts reports whether body asserts obj again.
func reasserts(pass *analysis.Pass, body []ast.Stmt, obj types.Object, typ types.Type) bool {
	var found bool
	for _, stmt := range body {
		ast.Inspect(stmt, func(n ast.Node) bool {
			assert, ok := n.(*ast.TypeAssertExpr)
			if !ok || assert.Type == nil {
				return true
			}
			if pass.TypesInfo.ObjectOf(ident(assert.X)) != obj {
				return true
			}
			if types.Identical(pass.TypesInfo.TypeOf(assert.Type), typ) {
				found = true
				return false
			}
			return true
		})
	}
	return found
}

// ident returns the identifier e names, or nil.
func ident(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}
