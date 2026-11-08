// Package testcmp provides an analysis.Analyzer that reports uses
// of reflect.DeepEqual in tests.
//
// DeepEqual is not configurable: unexported fields are always
// compared, a nil slice never equals an empty one, and NaN never
// equals itself. A comparator such as github.com/google/go-cmp
// lets a test say what equal means and shows what differed.
package testcmp

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "testcmp",
	Doc:  "reports uses of reflect.DeepEqual in tests",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		name := pass.Fset.File(f.Pos()).Name()
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && callee(pass, call) == "reflect.DeepEqual" {
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(),
					End: call.End(),
					Message: "avoid reflect.DeepEqual in " +
						"tests; consider go-cmp",
				})
			}
			return true
		})
	}
	return nil, nil
}

// callee returns the package-qualified name of a called package
// function.
func callee(pass *analysis.Pass, call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Imported().Path() + "." + sel.Sel.Name
}
