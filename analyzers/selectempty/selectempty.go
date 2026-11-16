// Package selectempty provides an analysis.Analyzer that reports a select
// statement with no cases.
//
// The select statements section of the Go spec says a select with no default
// case "blocks until at least one of the communications can proceed", and its
// example marks "select {}" with "block forever". An empty select therefore
// blocks the goroutine at that point. A reader cannot tell a deliberate wait
// from a select whose cases were dropped, so the rule reports every empty
// select.
//
// Report only: writing the missing cases changes what the code waits for, and
// the tool cannot tell which cases the author meant.
//
// The README catalogs every check and exemption.
//
// The rule is adapted from staticcheck's SA5004, which reports an empty
// default branch in a for-select loop.
package selectempty

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "selectempty",
	Doc:  "reports a select statement with no cases",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.SelectStmt)
			if ok && len(stmt.Body.List) == 0 {
				pass.Reportf(
					stmt.Pos(),
					"select with no cases blocks forever",
				)
			}
			return true
		})
	}
	return nil, nil
}
