// Package indexalloc provides an analysis.Analyzer that reports a search of a
// byte slice turned into a string.
//
// strings.Index(string(b), s) allocates a string that exists only to be
// searched and thrown away. bytes.Index(b, []byte(s)) allocates a byte slice,
// which the compiler can avoid for a literal. Go issue 25864 is about this
// shape. Like go-critic's indexAlloc.
package indexalloc

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "indexalloc",
	Doc:  "reports a byte slice searched as a string",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !isIndex(pass, call) {
				return true
			}
			if !isStringOfBytes(pass, call.Args[0]) {
				return true
			}
			pass.Reportf(call.Pos(), "string conversion only to search; use bytes.Index")
			return true
		})
	}
	return nil, nil
}

// isIndex reports whether call is strings.Index.
func isIndex(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Index" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == "strings"
}

// isStringOfBytes reports whether e is string(x) for a byte slice x.
func isStringOfBytes(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("string") {
		return false
	}
	typ := pass.TypesInfo.TypeOf(call.Args[0])
	slice, ok := types.Unalias(typ).(*types.Slice)
	if !ok {
		return false
	}
	basic, ok := slice.Elem().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Byte
}
