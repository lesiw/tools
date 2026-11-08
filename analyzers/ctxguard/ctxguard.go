// Package ctxguard provides an analysis.Analyzer for context
// discipline.
//
// One check: a function that opens with a ctx.Err() pre-check is
// reported.
// The first context-aware operation in the body will observe
// cancellation on its own; the pre-check is defensive noise that
// duplicates it. The init-statement form, if err := ctx.Err();
// err != nil, is the same pre-check. The fix removes the check,
// and a comment inside the pre-check is removed with it: it
// describes the deleted code.
// A loop condition, for ctx.Err() == nil, is the context-aware
// operation for its loop and is not reported.
package ctxguard

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ctxguard",
	Doc:  "reports redundant ctx.Err() pre-checks",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			var body *ast.BlockStmt
			switch node := n.(type) {
			case *ast.FuncDecl:
				body = node.Body
			case *ast.FuncLit:
				body = node.Body
			default:
				return true
			}
			if body != nil && len(body.List) > 0 {
				checkPrecheck(pass, body.List[0])
			}
			return true
		})
	}
	return nil, nil
}

// checkPrecheck reports stmt when it is an if whose only work is
// returning on ctx.Err(). The fix deletes the pre-check's lines;
// a comment inside them describes the deleted code, so it is
// removed along with it.
func checkPrecheck(pass *analysis.Pass, stmt ast.Stmt) {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || ifStmt.Else != nil || !onlyReturns(ifStmt.Body) {
		return
	}
	if !errPrecheck(pass, ifStmt) {
		return
	}
	var (
		edits []analysis.TextEdit
		file  = pass.Fset.File(stmt.Pos())
		line  = file.Line(stmt.End())
	)
	if line < file.LineCount() {
		edits = []analysis.TextEdit{{
			Pos: file.LineStart(file.Line(stmt.Pos())),
			End: file.LineStart(line + 1),
		}}
	}
	d := analysis.Diagnostic{
		Pos: ifStmt.Pos(),
		End: ifStmt.End(),
		Message: "redundant ctx.Err() pre-check: the first " +
			"context-aware operation observes cancellation",
	}
	if edits != nil {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the pre-check",
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// errPrecheck reports whether ifStmt tests ctx.Err() != nil, either
// directly or through an init assignment.
func errPrecheck(pass *analysis.Pass, ifStmt *ast.IfStmt) bool {
	cond, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.NEQ || !isNilIdent(cond.Y) {
		return false
	}
	if isCtxErrCall(pass, cond.X) {
		return ifStmt.Init == nil
	}
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return false
	}
	return isCtxErrCall(pass, assign.Rhs[0])
}

// isCtxErrCall reports whether e calls Err on a context.Context.
func isCtxErrCall(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Err" {
		return false
	}
	t := pass.TypesInfo.TypeOf(sel.X)
	return t != nil && t.String() == "context.Context"
}

// onlyReturns reports whether body is a single return statement.
func onlyReturns(body *ast.BlockStmt) bool {
	if len(body.List) != 1 {
		return false
	}
	_, ok := body.List[0].(*ast.ReturnStmt)
	return ok
}

func isNilIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}
