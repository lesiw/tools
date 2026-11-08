// Package testlabel provides an analysis.Analyzer that reports a subtest named
// by a literal.
//
// A subtest names a case in a family: it exists so a table's rows can be told
// apart in a failure message, or so a loop's iterations can. A literal name
// names one thing, so the subtest reads as a sentence about the code rather
// than a case in it, and a failure sends a reader looking for a subtest that
// has nothing to tell them.
package testlabel

import (
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "testlabel",
	Doc:  "reports a subtest whose name is a string literal",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !isRun(pass, call) {
				return true
			}
			if !isLiteral(pass, call.Args[0]) {
				return true
			}
			pass.Reportf(call.Args[0].Pos(), "subtest name is string literal")
			return true
		})
	}
	return nil, nil
}

// isRun reports whether call is a (*testing.T).Run.
func isRun(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" {
		return false
	}
	recv := pass.TypesInfo.Types[sel.X].Type
	if recv == nil {
		return false
	}
	if ptr, ok := recv.(*types.Pointer); ok {
		recv = ptr.Elem()
	}
	named, ok := recv.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "testing" &&
		obj.Name() == "T"
}

// isLiteral reports whether e is a string literal, or a constant that folds to
// one. A name read from a table row or built at run time is not a literal.
func isLiteral(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil {
		return false
	}
	return tv.Value.Kind() == constant.String
}
