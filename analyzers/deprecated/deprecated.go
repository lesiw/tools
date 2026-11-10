// Package deprecated provides an analysis.Analyzer that reports a doc comment
// that marks something deprecated without the form godoc reads.
//
// godoc and every editor that renders it look for a paragraph beginning with
// "Deprecated: ". A comment that opens with the word instead is prose, so the
// deprecation is invisible to the tools that would warn a caller.
package deprecated

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "deprecated",
	Doc:  "reports a deprecation that godoc will not read",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			doc := docOf(decl)
			if doc == nil || !opens(doc) || reads(doc) {
				continue
			}
			pass.Reportf(decl.Pos(), "deprecation without the \"Deprecated:\" prefix")
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

// opens reports whether the comment begins with the word deprecated.
func opens(cg *ast.CommentGroup) bool {
	var (
		text = strings.TrimSpace(cg.Text())
		word = strings.TrimRight(strings.SplitN(text, " ", 2)[0], ":,.;")
	)
	return strings.EqualFold(word, "deprecated")
}

// reads reports whether the comment has a paragraph godoc reads as a
// deprecation note.
func reads(cg *ast.CommentGroup) bool {
	for line := range strings.SplitSeq(cg.Text(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Deprecated: ") {
			return true
		}
	}
	return false
}
