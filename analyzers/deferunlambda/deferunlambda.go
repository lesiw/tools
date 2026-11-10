// Package deferunlambda provides an analysis.Analyzer that reports a deferred
// function literal that only calls another function.
//
// defer func() { f() }() delays the literal, and the literal calls f with no
// arguments. defer f() delays the call itself, which is one less function to
// read, and it takes the arguments at the same moment.
package deferunlambda

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "deferunlambda",
	Doc:  "reports a deferred literal that only calls a function",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.DeferStmt)
			if !ok || !wraps(stmt.Call) {
				return true
			}
			inner := stmt.Call.Fun.(*ast.FuncLit).Body.List[0].(*ast.ExprStmt)
			pass.Report(analysis.Diagnostic{
				Pos:     stmt.Pos(),
				End:     stmt.End(),
				Message: "deferred literal only calls a function; defer the call",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Defer the call",
					TextEdits: []analysis.TextEdit{{
						Pos:     stmt.Call.Pos(),
						End:     stmt.Call.End(),
						NewText: []byte(types.ExprString(inner.X)),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// wraps reports whether call is a function literal with one call in its body
// and no arguments of its own.
func wraps(call *ast.CallExpr) bool {
	lit, ok := call.Fun.(*ast.FuncLit)
	if !ok || len(call.Args) != 0 {
		return false
	}
	if lit.Type.Params != nil && len(lit.Type.Params.List) != 0 {
		return false
	}
	if len(lit.Body.List) != 1 {
		return false
	}
	expr, ok := lit.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	inner, ok := expr.X.(*ast.CallExpr)
	if !ok || len(inner.Args) != 0 {
		return false
	}
	// recover only stops a panic when a deferred function calls it.
	// defer recover() compiles, and it does not recover.
	if id, ok := inner.Fun.(*ast.Ident); ok && id.Name == "recover" {
		return false
	}
	return true
}
