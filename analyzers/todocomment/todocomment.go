// Package todocomment provides an analysis.Analyzer that reports a TODO,
// FIXME or BUG doc comment that says nothing.
//
// A bare // TODO gives the next reader nothing to act on. A search for the
// marker returns every note, finished or not. Like go-critic's
// todoCommentWithoutDetail, minus the assignee requirement.
package todocomment

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "todocomment",
	Doc:  "reports a TODO, FIXME or BUG doc comment with no detail",
	Run:  run,
}

// markers are the words that open a note.
var markers = []string{"TODO", "FIXME", "BUG"}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			doc := docOf(decl)
			if doc == nil || !bare(doc) {
				continue
			}
			pass.Reportf(decl.Pos(), "note with no detail; say what is missing")
		}
	}
	return nil, nil
}

// docOf returns the doc comment of a declaration.
func docOf(decl ast.Decl) *ast.CommentGroup {
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		return decl.Doc
	case *ast.GenDecl:
		return decl.Doc
	}
	return nil
}

// bare reports whether a line of the group is a marker with nothing after it.
func bare(group *ast.CommentGroup) bool {
	for line := range strings.SplitSeq(group.Text(), "\n") {
		body := strings.TrimSpace(line)
		for _, marker := range markers {
			rest, ok := strings.CutPrefix(body, marker)
			if !ok {
				continue
			}
			rest = strings.TrimSpace(rest)
			rest = strings.TrimPrefix(rest, "(")
			rest = strings.TrimSpace(rest)
			if rest == "" || rest == ")" {
				return true
			}
		}
	}
	return false
}
