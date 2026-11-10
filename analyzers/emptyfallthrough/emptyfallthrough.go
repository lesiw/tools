// Package emptyfallthrough provides an analysis.Analyzer that reports a case
// whose whole body is a fallthrough.
//
// case A: fallthrough followed by case B: body is the same program as
// case A, B: body, and the second form records that the two values share one
// body. Like go-critic's emptyFallthrough.
package emptyfallthrough

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptyfallthrough",
	Doc:  "reports a case whose whole body is a fallthrough",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			check(pass, stmt.Body)
			return true
		})
	}
	return nil, nil
}

// check reports each clause that only falls through into another case. A
// fallthrough into default is a different shape: the case is redundant
// outright, because an unmatched value goes to default anyway.
func check(pass *analysis.Pass, body *ast.BlockStmt) {
	for i, stmt := range body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok || len(clause.Body) != 1 || i == len(body.List)-1 {
			continue
		}
		branch, ok := clause.Body[0].(*ast.BranchStmt)
		if !ok || branch.Tok != token.FALLTHROUGH {
			continue
		}
		next, isCase := body.List[i+1].(*ast.CaseClause)
		if isCase && next.List == nil {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     branch.Pos(),
			End:     branch.End(),
			Message: "case only falls through; list both values in one case",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "List both values in one case",
				TextEdits: []analysis.TextEdit{{
					Pos:     clause.Colon,
					End:     next.List[0].Pos(),
					NewText: []byte(", "),
				}},
			}},
		})
	}
}
