// Package uselessbreak provides an analysis.Analyzer that reports a break at
// the end of a case clause in a switch.
//
// The break statement section of the spec says a break ends the innermost for,
// switch, or select (https://go.dev/ref/spec#Break_statements). The switch
// statements section says the statements of the selected case run and then the
// switch statement ends (https://go.dev/ref/spec#Switch_statements), so a break
// at the end of a case leaves a switch that would end anyway. A break with
// another statement after it skips that statement and is not reported. A
// switch inside a loop is reported too: the break leaves only the switch, so
// it does not leave the loop.
//
// An expression switch and a type switch both have case clauses. The check is
// limited to the statement that ends a case body, so a break inside a trailing
// if or block, or a break inside a labeled statement, is not checked. A select
// clause is not checked; select cases are outside this rule. A break at the
// end of a loop body is not reported either: it ends the loop, so deleting it
// can run the next pass.
//
// The analyzer reports a bare break only, even when a label names the switch
// itself. A labeled break can leave an enclosing statement, and deleting one
// whose label has no other use would leave the label without a use, which the
// compiler rejects. The analyzer skips every labeled break rather than decide
// which statement the label names.
//
// Safe fix: delete the break. The fix removes its line when the break is alone
// on it, and otherwise removes the break and leaves the rest of the line.
//
// Adapted from revive's useless-break, which also checks select clauses.
package uselessbreak

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "uselessbreak",
	Doc:  "reports a break at the end of a switch case",
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
			switch n := n.(type) {
			case *ast.SwitchStmt:
				check(pass, src, n.Body)
			case *ast.TypeSwitchStmt:
				check(pass, src, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the break at the end of each case in body.
func check(pass *analysis.Pass, src []byte, body *ast.BlockStmt) {
	for _, stmt := range body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok || len(clause.Body) == 0 {
			continue
		}
		branch, ok := clause.Body[len(clause.Body)-1].(*ast.BranchStmt)
		if !ok || branch.Tok != token.BREAK || branch.Label != nil {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     branch.Pos(),
			End:     branch.End(),
			Message: "break at the end of a switch case has no effect",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message:   "Delete the break",
				TextEdits: []analysis.TextEdit{removal(pass, src, branch)},
			}},
		})
	}
}

// removal returns the edit that deletes branch. When the break is alone on its
// line, the fix deletes the whole line; otherwise it deletes only the break.
func removal(pass *analysis.Pass, src []byte, branch *ast.BranchStmt) analysis.TextEdit {
	var (
		tokFile    = pass.Fset.File(branch.Pos())
		start, end = tokFile.Offset(branch.Pos()), tokFile.Offset(branch.End())
	)
	if source.Alone(src, start, end) {
		return analysis.TextEdit{
			Pos: tokFile.Pos(source.Start(src, start)),
			End: tokFile.Pos(source.End(src, end)),
		}
	}
	return analysis.TextEdit{Pos: branch.Pos(), End: branch.End()}
}
