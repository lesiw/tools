// Package unslice provides an analysis.Analyzer that reports a full slice
// expression applied to the value it slices.
//
// s[:] is s. The expression reads as a copy or a conversion and does neither.
// Like go-critic's unslice.
package unslice

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unslice",
	Doc:  "reports a slice expression that takes the whole value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			slice, ok := n.(*ast.SliceExpr)
			if !ok || slice.Low != nil || slice.High != nil {
				return true
			}
			if slice.Max != nil {
				return true
			}
			// Slicing an array converts it to a slice, which is not
			// the same value.
			if !already(pass, slice.X) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     slice.Pos(),
				End:     slice.End(),
				Message: "unnecessary full slice",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Use the value",
					TextEdits: []analysis.TextEdit{{
						Pos: slice.Lbrack,
						End: slice.Rbrack + 1,
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// already reports whether e is a slice or a string, where s[:] is s.
func already(pass *analysis.Pass, e ast.Expr) bool {
	switch types.Unalias(pass.TypesInfo.TypeOf(e)).Underlying().(type) {
	case *types.Slice, *types.Basic:
		return true
	}
	return false
}
