// Package errflow provides an analysis.Analyzer that reports an else block
// that follows a terminating statement.
//
// Go keeps the normal path unindented and indents the error handling. A block
// that ends in return, break, continue or goto has already left the function
// or the loop, so its else runs only when the block did not. The else adds an
// indent to code that reads better without one.
package errflow

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errflow",
	Doc:  "reports an else block that follows a terminating statement",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok || ifStmt.Else == nil {
				return true
			}
			// An else if is a chain, and a chain reads better than a
			// column of outdented ifs.
			if _, ok := ifStmt.Else.(*ast.IfStmt); ok {
				return true
			}
			// With an init statement the else block can name values the
			// outdented form would not.
			if ifStmt.Init != nil {
				return true
			}
			if !terminates(pass, ifStmt.Body) {
				return true
			}
			pass.Reportf(ifStmt.Else.Pos(), "unnecessary else")
			return true
		})
	}
	return nil, nil
}

// terminates reports whether control cannot reach the end of block.
func terminates(pass *analysis.Pass, block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	switch stmt := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok != token.FALLTHROUGH
	case *ast.ExprStmt:
		return isPanic(pass, stmt.X)
	case *ast.IfStmt:
		// An if that terminates both ways terminates the block.
		return stmt.Else != nil && terminates(pass, stmt.Body) &&
			terminates(pass, asBlock(stmt.Else))
	}
	return false
}

// asBlock returns the body of an else statement, if it has one.
func asBlock(stmt ast.Stmt) *ast.BlockStmt {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		return stmt
	case *ast.IfStmt:
		return &ast.BlockStmt{List: []ast.Stmt{stmt}}
	}
	return nil
}

// isPanic reports whether e is a call to the builtin panic.
func isPanic(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	return pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic")
}
