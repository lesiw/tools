// Package commentfmt provides an analysis.Analyzer that reports a comment with
// no space after its slashes.
//
// //comment reads as code that was commented out, and every editor highlights
// it as such. A space after the slashes is what makes it prose. A directive
// such as //go:generate or //nolint:foo has no space by design, so a comment
// whose first word carries a colon is left alone. Like go-critic's
// commentFormatting.
package commentfmt

import (
	"go/ast"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "commentfmt",
	Doc:  "reports a comment with no space after its slashes",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		docs := docGroups(file)
		for _, group := range file.Comments {
			if docs[group] {
				continue // gofmt sets the space in a doc comment
			}
			for _, comment := range group.List {
				if bad(comment.Text) {
					pass.Report(analysis.Diagnostic{
						Pos:     comment.Pos(),
						End:     comment.End(),
						Message: "comment has no space after //",
						SuggestedFixes: []analysis.SuggestedFix{{
							Message: "Add the space",
							TextEdits: []analysis.TextEdit{{
								Pos:     comment.Pos(),
								End:     comment.Pos() + 2,
								NewText: []byte("// "),
							}},
						}},
					})
				}
			}
		}
	}
	return nil, nil
}

// docGroups returns the comment groups that document a declaration. gofmt
// writes the space after // in these itself, so a report would name a change
// the formatter already makes.
func docGroups(file *ast.File) map[*ast.CommentGroup]bool {
	var (
		docs = make(map[*ast.CommentGroup]bool)
		add  = func(cg *ast.CommentGroup) {
			if cg != nil {
				docs[cg] = true
			}
		}
	)
	add(file.Doc)
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			add(decl.Doc)
		case *ast.GenDecl:
			add(decl.Doc)
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					add(spec.Doc)
					add(spec.Comment)
				case *ast.ValueSpec:
					add(spec.Doc)
					add(spec.Comment)
				case *ast.ImportSpec:
					add(spec.Doc)
					add(spec.Comment)
				}
			}
		}
	}
	return docs
}

// directives are the bare words a line comment may open with.
var directives = map[string]bool{
	"line":   true,
	"export": true,
	"extern": true,
	"sys":    true,
	"sysnb":  true,
	"nolint": true,
}

// bad reports whether text is a line comment that should have a space.
func bad(text string) bool {
	if !strings.HasPrefix(text, "//") {
		return false // a block comment, or something else
	}
	rest := text[2:]
	if rest == "" || strings.HasPrefix(rest, " ") {
		return false
	}
	if strings.HasPrefix(rest, "\t") {
		return false
	}
	// A directive names a tool and ends in a colon before any space, or is
	// one of the bare words the toolchain reads.
	word := rest
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		word = rest[:i]
	}
	if strings.Contains(word, ":") || directives[word] {
		return false
	}
	// A comment that opens with punctuation is a separator someone drew,
	// not a sentence.
	first, _ := utf8.DecodeRuneInString(rest)
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		return false
	}
	return true
}
