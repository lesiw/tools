// Package makezero provides an analysis.Analyzer that reports a slice made
// with a non-zero length that is only ever appended to.
//
// make([]T, n) allocates n elements and len n, so an append after it writes
// past the zeros and leaves them in the result. The caller that wanted a
// buffer meant make([]T, 0, n), which keeps the allocation and starts the
// length at zero.
package makezero

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "makezero",
	Doc:  "reports a slice made with a non-zero length that is appended to",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			check(pass, fn.Body)
		}
	}
	return nil, nil
}

// check reports each make whose slice is later appended to.
func check(pass *analysis.Pass, body *ast.BlockStmt) {
	made := make(map[types.Object]*ast.CallExpr)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if obj, call, ok := declaresSlice(pass, n); ok {
				made[obj] = call
			}
		case *ast.CallExpr:
			obj, ok := appended(pass, n)
			if !ok {
				return true
			}
			if call, ok := made[obj]; ok {
				pass.Report(analysis.Diagnostic{
					Pos:     call.Pos(),
					End:     call.End(),
					Message: "slice made with a length, then appended to",
					SuggestedFixes: []analysis.SuggestedFix{{
						Message: "Give the slice a zero length",
						TextEdits: []analysis.TextEdit{{
							Pos:     call.Args[1].Pos(),
							End:     call.Args[1].Pos(),
							NewText: []byte("0, "),
						}},
					}},
				})
				delete(made, obj)
			}
		}
		return true
	})
}

// declaresSlice returns the object a statement declares with a make call, and
// the call, when the call gives the slice a non-zero length.
func declaresSlice(pass *analysis.Pass, stmt *ast.AssignStmt) (types.Object, *ast.CallExpr, bool) {
	if stmt.Tok != token.DEFINE || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return nil, nil, false
	}
	call, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok || !nonZeroSlice(pass, call) {
		return nil, nil, false
	}
	id, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	return pass.TypesInfo.Defs[id], call, true
}

// appended returns the object an append call extends.
func appended(pass *analysis.Pass, call *ast.CallExpr) (types.Object, bool) {
	if len(call.Args) == 0 {
		return nil, false
	}
	builtin, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[builtin] != types.Universe.Lookup("append") {
		return nil, false
	}
	first, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return nil, false
	}
	return pass.TypesInfo.Uses[first], true
}

// nonZeroSlice reports whether call is make([]T, n) with a non-zero constant
// length.
func nonZeroSlice(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 2 {
		return false
	}
	builtin, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[builtin] != types.Universe.Lookup("make") {
		return false
	}
	slice, ok := call.Args[0].(*ast.ArrayType)
	if !ok || slice.Len != nil {
		return false
	}
	lit, ok := call.Args[1].(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	return lit.Value != "0"
}
