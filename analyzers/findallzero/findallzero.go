// Package findallzero provides an analysis.Analyzer that reports a FindAll
// call on a regexp.Regexp with a count of zero.
//
// A FindAll method returns at most n matches when n >= 0, and every match
// otherwise, so a count of zero never returns a match.
//
// A fix would guess the count the author meant, so the analyzer reports
// without a fix.
//
// The rule is like staticcheck's SA1010.
package findallzero

import (
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "findallzero",
	Doc:  "reports a FindAll call on a regexp.Regexp with a count of zero",
	Run:  run,
}

// findalls take n as their last argument.
var findalls = map[string]struct{}{
	"FindAll":                    {},
	"FindAllIndex":               {},
	"FindAllString":              {},
	"FindAllStringIndex":         {},
	"FindAllStringSubmatch":      {},
	"FindAllStringSubmatchIndex": {},
	"FindAllSubmatch":            {},
	"FindAllSubmatchIndex":       {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if arg := zeroArg(pass, call); arg != nil {
					pass.Reportf(arg.Pos(), "a count of zero always returns no matches")
				}
			}
			return true
		})
	}
	return nil, nil
}

// zeroArg returns the last argument of a regexp FindAll call when it is the
// constant zero.
func zeroArg(pass *analysis.Pass, call *ast.CallExpr) ast.Expr {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil
	}
	// A method of a predeclared interface, such as error.Error, has no
	// package.
	if fn.Pkg() == nil || fn.Pkg().Path() != "regexp" {
		return nil
	}
	if _, ok := findalls[fn.Name()]; !ok {
		return nil
	}
	if len(call.Args) == 0 {
		return nil
	}
	// A call passes the count last, and so does a method expression, which
	// passes the receiver first.
	arg := call.Args[len(call.Args)-1]
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return nil
	}
	if constant.Sign(tv.Value) == 0 {
		return arg
	}
	return nil
}
