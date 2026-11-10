// Package regexpmust provides an analysis.Analyzer that reports a regexp
// compiled from a constant whose error is thrown away.
//
// A pattern that is a literal either compiles or never will, so the program
// cannot handle the error at run time. regexp.MustCompile does the same and
// panics at init, which is where a bad constant belongs.
package regexpmust

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "regexpmust",
	Doc:  "reports a regexp compiled from a constant with its error dropped",
	Run:  run,
}

// compiled are the functions whose error is dropped when a constant fails.
var compiled = map[string]bool{"Compile": true, "CompilePOSIX": true}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isCompile(pass, call) || !isLiteral(call) {
				return true
			}
			pass.Reportf(call.Pos(), "regexp compiled from a constant; use MustCompile")
			return true
		})
	}
	return nil, nil
}

// isCompile reports whether call is regexp.Compile or regexp.CompilePOSIX.
func isCompile(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !compiled[sel.Sel.Name] {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == "regexp"
}

// isLiteral reports whether the pattern is a string literal.
func isLiteral(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}
