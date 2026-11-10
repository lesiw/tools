// Package zerorepeat provides an analysis.Analyzer that reports a run of
// zero bytes built with bytes.Repeat.
//
// bytes.Repeat([]byte{0}, n) allocates the pattern and then the result.
// make([]byte, n) allocates the result and leaves it zeroed, which is the
// value the call was building. Like go-critic's zeroByteRepeat.
package zerorepeat

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "zerorepeat",
	Doc:  "reports a run of zero bytes built with bytes.Repeat",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !isRepeat(pass, call) {
				return true
			}
			if !allZero(pass, call.Args[0]) {
				return true
			}
			pass.Reportf(call.Pos(), "zero bytes from bytes.Repeat; use make([]byte, n)")
			return true
		})
	}
	return nil, nil
}

// isRepeat reports whether call is bytes.Repeat.
func isRepeat(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Repeat" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == "bytes"
}

// allZero reports whether e is a byte slice literal of zeroes.
func allZero(_ *analysis.Pass, e ast.Expr) bool {
	lit, ok := e.(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 1 {
		return false
	}
	basic, ok := lit.Elts[0].(*ast.BasicLit)
	if !ok {
		return false
	}
	return basic.Value == "0"
}
