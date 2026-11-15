// Package unnecessarydefer provides an analysis.Analyzer that reports a
// deferred call that can be replaced by a plain call.
//
// The Go spec's Defer statements section says a deferred call runs immediately
// before the surrounding function returns, so a call that ends the function
// runs at the same moment either way.
//
// The idea comes from go-critic's unnecessaryDefer.
//
// Report only: the edit changes the line a callee reads from its caller's
// frame. The README catalogs every check and exemption.
package unnecessarydefer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unnecessarydefer",
	Doc:  "reports a deferred call that can be replaced by a plain call",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				reportTails(pass, n.Body)
			case *ast.FuncLit:
				reportTails(pass, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// reportTails reports every defer in body that can run last as a plain call.
func reportTails(pass *analysis.Pass, body *ast.BlockStmt) {
	if body == nil {
		return // an assembly stub declares a function without a body
	}
	for _, deferStmt := range listTails(body.List) {
		if !canBePlain(pass, deferStmt.Call) {
			continue
		}
		pass.Reportf(deferStmt.Pos(), "deferred call can be replaced by a plain call")
	}
}

// listTails returns the defers in list that can run as its last action. A
// trailing return that lists no results is skipped, because the statement
// before it runs last.
func listTails(list []ast.Stmt) []*ast.DeferStmt {
	if len(list) == 0 {
		return nil
	}
	last := unlabel(list[len(list)-1])
	if ret, ok := last.(*ast.ReturnStmt); ok {
		if len(ret.Results) != 0 {
			return nil
		}
		return listTails(list[:len(list)-1])
	}
	return stmtTails(last)
}

// unlabel returns the statement that stmt's labels wrap.
func unlabel(stmt ast.Stmt) ast.Stmt {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = labeled.Stmt
	}
}

// stmtTails returns the defers that can run last when stmt ends a body or a
// clause of a switch or select body. A loop is left alone: its deferred calls
// run after the loop, where a plain call would run at the end of each pass.
func stmtTails(stmt ast.Stmt) []*ast.DeferStmt {
	switch stmt := stmt.(type) {
	case *ast.DeferStmt:
		return []*ast.DeferStmt{stmt}
	case *ast.BlockStmt:
		return listTails(stmt.List)
	case *ast.CaseClause:
		return listTails(stmt.Body)
	case *ast.CommClause:
		return listTails(stmt.Body)
	case *ast.IfStmt:
		out := listTails(stmt.Body.List)
		if stmt.Else != nil {
			out = append(out, stmtTails(stmt.Else)...)
		}
		return out
	case *ast.SwitchStmt:
		return clauseTails(stmt.Body.List)
	case *ast.TypeSwitchStmt:
		return clauseTails(stmt.Body.List)
	case *ast.SelectStmt:
		return clauseTails(stmt.Body.List)
	}
	return nil
}

// clauseTails returns the defers that can run last in the clauses of a switch
// or a select, since any one clause can be the one that runs.
func clauseTails(clauses []ast.Stmt) []*ast.DeferStmt {
	out := make([]*ast.DeferStmt, 0, len(clauses))
	for _, clause := range clauses {
		out = append(out, stmtTails(clause)...)
	}
	return out
}

// canBePlain reports whether call could stand without the keyword. A literal
// is left to deferunlambda. A result would be left unread by the plain call,
// which the suite reports as an unchecked error or an unused result.
func canBePlain(pass *analysis.Pass, call *ast.CallExpr) bool {
	if _, ok := ast.Unparen(call.Fun).(*ast.FuncLit); ok {
		return false
	}
	sig, ok := pass.TypesInfo.TypeOf(call.Fun).Underlying().(*types.Signature)
	return ok && sig.Results().Len() == 0
}
