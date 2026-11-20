// Package selectone provides an analysis.Analyzer that reports a select
// statement with a single case clause and no default.
//
// The select statements section of the Go spec says a select with no default
// "blocks until at least one of the communications can proceed"
// (https://go.dev/ref/spec#Select_statements). With one case, that is the send
// or receive in the case, which blocks the same way. Removing the select
// leaves the send or receive and the body.
//
// A select with no cases, a default clause, or more than one case is left
// alone. A select whose case body holds a break that leaves the select is
// left alone too: the break statement section says a break ends the innermost
// for, switch, or select, or the statement named by a label
// (https://go.dev/ref/spec#Break_statements), and removing the select would
// change where the break goes. A select whose case discards a value
// received from time.After is left to the selectsleep analyzer.
//
// Safe fix: replace the select with the send or receive in the case and the
// body. The fix is withheld when it would delete a comment, when the select
// does not occupy whole lines, when the body holds a raw string that spans
// lines, when a name in the case is declared anywhere in the block around the
// select or in a scope that block can see, when a goto would jump over a var
// declaration the fix moves out of the case, and when a sibling select would
// move a name into that block too.
//
// The rule is adapted from staticcheck's S1000, which reports a single-case
// select and offers no fix.
package selectone

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/singlecase"
	"lesiw.io/tools/internal/timeexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "selectone",
	Doc:  "reports a select statement with a single case and no default",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectStmt)
			if !ok {
				return true
			}
			clause, ok := singlecase.Single(sel)
			if !ok {
				return true
			}
			if singlecase.Leaves(file, sel, clause) {
				return true
			}
			// selectsleep reports a wait on time.After.
			if _, _, ok := timeexpr.AfterCall(pass, clause.Comm); ok {
				return true
			}
			pass.Report(diagnostic(pass, src, file, sel, clause))
			return true
		})
	}
	return nil, nil
}

// diagnostic returns the report for one select, with a fix when the select can
// be removed without changing the program.
func diagnostic(pass *analysis.Pass, src []byte, file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) analysis.Diagnostic {
	d := analysis.Diagnostic{
		Pos:     sel.Pos(),
		End:     sel.End(),
		Message: "select with a single case is redundant",
	}
	if edit, ok := singlecase.Unwrap(pass, src, file, sel, clause); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the select",
			TextEdits: []analysis.TextEdit{edit},
		}}
	}
	return d
}
