// Package duplarg provides an analysis.Analyzer that reports a call that
// passes one value where it expects two.
//
// f(x, x) is a question with one answer: strings.Contains(x, x) is always
// true, math.Max(x, x) is x, and copy(x, x) copies a slice onto itself. The
// call reads as if two different values were meant. Like go-critic's dupArg.
package duplarg

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "duplarg",
	Doc:  "reports a call that passes one value where it expects two",
	Run:  run,
}

// known are the functions whose answer cannot depend on two identical
// arguments.
var known = map[string]bool{
	"bytes.Equal":       true,
	"cmp.Compare":       true,
	"copy":              true,
	"math.Dim":          true,
	"math.Max":          true,
	"math.Min":          true,
	"reflect.Copy":      true,
	"reflect.DeepEqual": true,
	"slices.Compare":    true,
	"slices.Equal":      true,
	"strings.Compare":   true,
	"strings.Contains":  true,
	"strings.EqualFold": true,
	"strings.HasPrefix": true,
	"strings.HasSuffix": true,
	"strings.Index":     true,
	"strings.LastIndex": true,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			if !same(pass, call.Args[0], call.Args[1]) {
				return true
			}
			if !known[name(pass, call)] {
				return true
			}
			pass.Reportf(call.Pos(), "both arguments are the same value")
			return true
		})
	}
	return nil, nil
}

// name returns the name a call's function is written as.
func name(pass *analysis.Pass, call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		ident, ok := fun.X.(*ast.Ident)
		if !ok {
			return ""
		}
		pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
		if !ok {
			return ""
		}
		return pkg.Imported().Path() + "." + fun.Sel.Name
	}
	return ""
}

// same reports whether two arguments are the same value. Only plain
// identifiers are compared, so a call with a side effect is left alone.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	x, ok := a.(*ast.Ident)
	if !ok {
		return false
	}
	y, ok := b.(*ast.Ident)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(x)
	return obj != nil && obj == pass.TypesInfo.ObjectOf(y)
}
