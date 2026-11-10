// Package typeunparen provides an analysis.Analyzer that reports redundant
// parentheses in a type expression.
//
// A parenthesized type is the same type, and the parentheses make a reader
// stop to check whether something else is going on. gofmt already strips them
// from parameter and result types; this covers the positions it leaves alone,
// such as a var or type declaration and a map value. Like go-critic's
// typeUnparen.
package typeunparen

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "typeunparen",
	Doc:  "reports redundant parentheses in a type expression",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// A parenthesized type in front of a call is a conversion, and
		// the parentheses are what make it parse. A parenthesized type
		// in front of a selector is a method expression, and the
		// parentheses are load bearing there too.
		needed := make(map[*ast.ParenExpr]struct{})
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if paren, ok := n.Fun.(*ast.ParenExpr); ok {
					needed[paren] = struct{}{}
				}
			case *ast.SelectorExpr:
				if paren, ok := n.X.(*ast.ParenExpr); ok {
					needed[paren] = struct{}{}
				}
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			var (
				paren, ok = n.(*ast.ParenExpr)
				required  bool
			)
			if ok {
				_, required = needed[paren]
			}
			if !ok || required || !isType(pass, paren.X) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     paren.Pos(),
				End:     paren.End(),
				Message: "redundant parentheses around a type",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Drop the parentheses",
					TextEdits: []analysis.TextEdit{{
						Pos:     paren.Pos(),
						End:     paren.End(),
						NewText: []byte(types.ExprString(paren.X)),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// isType reports whether e is a type expression.
func isType(pass *analysis.Pass, e ast.Expr) bool {
	if tv, ok := pass.TypesInfo.Types[e]; ok && tv.IsType() {
		return true
	}
	switch e := e.(type) {
	case *ast.Ident:
		_, ok := pass.TypesInfo.Uses[e].(*types.TypeName)
		return ok
	case *ast.SelectorExpr:
		_, ok := pass.TypesInfo.Uses[e.Sel].(*types.TypeName)
		return ok
	case *ast.StarExpr:
		return isType(pass, e.X)
	case *ast.ArrayType, *ast.StructType, *ast.InterfaceType,
		*ast.MapType, *ast.ChanType, *ast.FuncType:
		return true
	}
	return false
}
