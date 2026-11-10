// Package elseif provides an analysis.Analyzer that reports an if nested in
// an else block.
//
// else { if ... } and else if ... are the same program. The second form
// removes a pair of braces and one level of indent. Like go-critic's elseif.
package elseif

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "elseif",
	Doc:  "reports an if nested in an else block",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			block, ok := stmt.Else.(*ast.BlockStmt)
			if !ok || len(block.List) != 1 {
				return true
			}
			if _, ok := block.List[0].(*ast.IfStmt); !ok {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     stmt.Else.Pos(),
				End:     block.End(),
				Message: "if nested in an else; write else if",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Write else if",
					TextEdits: []analysis.TextEdit{{
						Pos: block.Lbrace,
						End: block.List[0].Pos(),
					}, {
						Pos: block.List[0].End(),
						End: block.Rbrace + 1,
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}
