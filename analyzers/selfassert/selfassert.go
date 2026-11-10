// Package selfassert provides an analysis.Analyzer that reports a type
// assertion whose source and destination types are the same.
//
// An assertion from a type to itself always succeeds and yields the value it
// was given, so the assertion is a no-op that reads as a conversion. Like
// go-critic's sloppyTypeAssert, whose name this package does not keep.
package selfassert

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "selfassert",
	Doc:  "reports a type assertion from a type to itself",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			assert, ok := n.(*ast.TypeAssertExpr)
			if !ok || assert.Type == nil {
				return true
			}
			from := pass.TypesInfo.TypeOf(assert.X)
			to := pass.TypesInfo.TypeOf(assert)
			if from == nil || to == nil || !types.Identical(from, to) {
				return true
			}
			pass.Reportf(assert.Pos(), "type assertion from a type to itself")
			return true
		})
	}
	return nil, nil
}
