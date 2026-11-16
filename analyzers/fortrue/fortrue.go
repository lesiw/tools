// Package fortrue provides an analysis.Analyzer that reports a for statement
// whose condition is the literal true and whose for clause has no init or post
// statement.
//
// The For statements section of the spec says the condition may be omitted, in
// which case it is equivalent to true
// (https://go.dev/ref/spec#For_statements). A loop whose condition is written
// as true runs the same way as the bare loop.
//
// The rule is limited to a loop with no init or post statement. The fix
// replaces the whole header, which would remove an init or post statement. A
// name that shadows true, a named constant whose value is true, and a comment
// in the loop header are left alone.
//
// The rule is adapted from staticcheck's S1006.
package fortrue

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "fortrue",
	Doc:  "reports a for statement whose condition is the literal true and whose for clause has no init or post statement",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if loop, ok := n.(*ast.ForStmt); ok {
				check(pass, file, loop)
			}
			return true
		})
	}
	return nil, nil
}

// check reports loop when its condition is the literal true, its for clause
// has no init or post statement, and no comment sits in the header.
func check(pass *analysis.Pass, file *ast.File, loop *ast.ForStmt) {
	if loop.Init != nil || loop.Post != nil || !isTrue(pass, loop.Cond) {
		return
	}
	// The edit includes the whole header, since it may hold parentheses or
	// semicolons besides the condition.
	start, end := loop.For+3, loop.Body.Lbrace
	if commented(file.Comments, start, end) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     loop.For,
		End:     loop.Cond.End(),
		Message: "loop condition is true; drop it",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Drop the condition",
			TextEdits: []analysis.TextEdit{{
				Pos:     start,
				End:     end,
				NewText: []byte(" "),
			}},
		}},
	})
}

// isTrue reports whether e is the predeclared true, with any parentheses
// removed.
func isTrue(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("true")
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
