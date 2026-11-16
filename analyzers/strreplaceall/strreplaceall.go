// Package strreplaceall provides an analysis.Analyzer that reports a
// strings.Replace call with a count of -1.
//
// The strings.Replace doc gives the count as the number of replacements and
// states that a negative count removes the limit
// (https://pkg.go.dev/strings#Replace). ReplaceAll replaces every occurrence
// (https://pkg.go.dev/strings#ReplaceAll), so the count of -1 adds an argument
// and nothing else.
//
// The count has to be written as the negative literal -1. A count in a
// constant or a variable is left alone, as is a call through a function value.
// A call written without the package name is left alone, as in a dot import.
// The fix renames the call and deletes the count, and is withheld where it
// would delete a comment.
//
// The rule is adapted from staticcheck's QF1004.
//
// The README catalogs every check, exemption, and fix.
package strreplaceall

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "strreplaceall",
	Doc:  "reports a strings.Replace call with a count of -1",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if diag, ok := diagnose(pass, file, call); ok {
					pass.Report(diag)
				}
			}
			return true
		})
	}
	return nil, nil
}

// diagnose returns the report for a strings.Replace call with a count of -1,
// and its fix where the count can be deleted without a comment.
func diagnose(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (analysis.Diagnostic, bool) {
	if len(call.Args) != 4 || !minusOne(call.Args[3]) {
		return analysis.Diagnostic{}, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.FullName() != "strings.Replace" {
		return analysis.Diagnostic{}, false
	}
	diag := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "Replace with a count of -1; use ReplaceAll",
	}
	edits := []analysis.TextEdit{{
		Pos:     sel.Sel.Pos(),
		End:     sel.Sel.End(),
		NewText: []byte("ReplaceAll"),
	}}
	start, end := call.Args[2].End(), call.Args[3].End()
	if commented(file, start, end) {
		return diag, true
	}
	edits = append(edits, analysis.TextEdit{Pos: start, End: end})
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message:   "Use ReplaceAll",
		TextEdits: edits,
	}}
	return diag, true
}

// minusOne reports whether e is the negative integer literal -1.
func minusOne(e ast.Expr) bool {
	unary, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB {
		return false
	}
	lit, ok := ast.Unparen(unary.X).(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "1"
}

// commented reports whether a comment sits in the span a fix would delete.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
