// Package selectsleep provides an analysis.Analyzer that reports a select
// statement with a single case that receives from time.After and discards the
// value.
//
// The time.After doc says "After waits for the duration to elapse and then
// sends the current time on the returned channel"
// (https://pkg.go.dev/time#After), and the select statements section of the Go
// spec says a select with no default "blocks until at least one of the
// communications can proceed"
// (https://go.dev/ref/spec#Select_statements). A select with one case and no
// default therefore blocks for the duration and then discards the time it
// receives, so time.Sleep(d) writes the same wait in one statement.
//
// A select with a second case is left alone, because it waits for whichever
// case arrives first. So is a select with a default clause, which returns at
// once when the timer has not fired, and a case that stores the received time,
// since time.Sleep returns nothing.
//
// Safe fix: the select is replaced with time.Sleep(d), written with the import
// name the file uses for the time package, so an import named t yields
// t.Sleep(d), followed by the case body. A case with no body is replaced with
// the call alone. A case with a body is fixed only when moving its statements
// into the block around the select cannot change the program: no name in the
// case is declared where that block can see it, the body holds no break that
// leaves the select and no raw string that spans lines, no goto jumps over a
// declaration the fix moves, the select occupies whole lines, and no sibling
// select moves one of those names too. The fix is withheld for a dot import as
// well, where there is no import name to write, and when a comment outside the
// call would be deleted. The select is still reported in every one of those
// cases.
//
// The rule is adapted from staticcheck's S1037, which reports a select that
// receives from time.After and offers a fix for the empty case.
package selectsleep

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/singlecase"
	"lesiw.io/tools/internal/timeexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "selectsleep",
	Doc:  "reports a select statement with a single case that receives from time.After and discards the value",
	Run:  run,
}

// finding is a select whose only case discards a value from time.After. The
// qualifier is the name the file uses for the time package, or empty for a dot
// import.
type finding struct {
	sel    *ast.SelectStmt
	clause *ast.CommClause
	call   *ast.CallExpr
	qual   string
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
			if f, ok := afterCase(pass, sel); ok {
				pass.Report(f.diagnose(pass, src, file))
			}
			return true
		})
	}
	return nil, nil
}

// afterCase returns the finding for sel when its only case discards a value
// from time.After.
func afterCase(pass *analysis.Pass, sel *ast.SelectStmt) (finding, bool) {
	clause, ok := singlecase.Single(sel)
	if !ok {
		return finding{}, false
	}
	call, qual, ok := timeexpr.AfterCall(pass, clause.Comm)
	if !ok {
		return finding{}, false
	}
	return finding{sel: sel, clause: clause, call: call, qual: qual}, true
}

// diagnose returns the report for the finding, with a fix when the replacement
// preserves the program.
func (f finding) diagnose(pass *analysis.Pass, src []byte, file *ast.File) analysis.Diagnostic {
	diag := analysis.Diagnostic{
		Pos:     f.sel.Pos(),
		End:     f.sel.End(),
		Message: "select with a single case that receives from time.After",
	}
	if f.qual == "" {
		return diag
	}
	tok := pass.Fset.File(f.sel.Pos())
	args := src[tok.Offset(f.call.Lparen)+1 : tok.Offset(f.call.Rparen)]
	call := f.qual + ".Sleep(" + string(args) + ")"
	edit, ok := f.edit(pass, src, file, call)
	if !ok {
		return diag
	}
	diag.Message += "; write " + call
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message:   "Write " + call,
		TextEdits: []analysis.TextEdit{edit},
	}}
	return diag
}

// edit returns the edit that replaces the select with call and the case body,
// and false when the edit would lose a comment or change the program.
func (f finding) edit(pass *analysis.Pass, src []byte, file *ast.File, call string) (analysis.TextEdit, bool) {
	if len(f.clause.Body) != 0 {
		return singlecase.Edit(pass, src, file, f.sel, f.clause, call)
	}
	if lostComment(file, f) {
		return analysis.TextEdit{}, false
	}
	return analysis.TextEdit{
		Pos:     f.sel.Pos(),
		End:     f.sel.End(),
		NewText: []byte(call),
	}, true
}

// lostComment reports whether a comment outside the argument list sits where
// the call takes the select's place.
func lostComment(file *ast.File, f finding) bool {
	return astexpr.Commented(file, f.sel.Pos(), f.call.Lparen) ||
		astexpr.Commented(file, f.call.Rparen, f.sel.End())
}
