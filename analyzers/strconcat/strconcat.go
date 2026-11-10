// Package strconcat provides an analysis.Analyzer that reports a two- or
// three-element Join with a literal separator.
//
// strings.Join allocates a slice for its arguments and then walks it. For two
// or three parts, concatenation allocates once and needs no slice literal in
// the middle of the expression. Like go-critic's stringConcatSimplify.
package strconcat

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "strconcat",
	Doc:  "reports a two- or three-element Join with a literal separator",
	Run:  run,
}

// maxParts is the number of parts a concatenation stays readable at.
const maxParts = 3

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !joins(pass, call) || !simple(call) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos: call.Pos(),
				End: call.End(),
				Message: "Join of " + strconv.Itoa(len(parts(call))) +
					" parts; concatenate them",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Concatenate the parts",
					TextEdits: []analysis.TextEdit{{
						Pos:     call.Pos(),
						End:     call.End(),
						NewText: []byte(joined(call)),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// joins reports whether call is strings.Join.
func joins(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 2 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == "strings"
}

// simple reports whether the call has the shape a concatenation can express:
// a slice literal of a few parts and a string literal separator.
func simple(call *ast.CallExpr) bool {
	lit, ok := call.Args[1].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	parts := parts(call)
	if len(parts) < 2 || len(parts) > maxParts {
		return false
	}
	// A keyed element has no place in a concatenation.
	for _, part := range parts {
		if _, ok := part.(*ast.KeyValueExpr); ok {
			return false
		}
	}
	return true
}

// joined renders the concatenation the call stands for.
func joined(call *ast.CallExpr) string {
	var (
		sep  = call.Args[1].(*ast.BasicLit).Value
		join = " + "
		b    strings.Builder
	)
	if sep != `""` {
		join = " + " + sep + " + "
	}
	for i, part := range parts(call) {
		if i > 0 {
			b.WriteString(join)
		}
		b.WriteString(types.ExprString(part))
	}
	return b.String()
}

// parts returns the elements of the call's slice literal.
func parts(call *ast.CallExpr) []ast.Expr {
	slice, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return nil
	}
	return slice.Elts
}
