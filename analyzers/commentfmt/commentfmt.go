// Package commentfmt provides an analysis.Analyzer that reports a comment with
// no space after its slashes.
//
// //comment looks like code that was commented out, and every editor
// highlights it that way. A space after the slashes makes it prose. The rule
// matches go-critic's commentFormatting.
//
// A directive is left alone, and the definition is the printer's: "line ",
// "extern " or "export ", or a word of lowercase letters and digits, a colon,
// and a lowercase letter or digit. Three bare words a tool reads with no colon
// — //sys, //sysnb and //nolint — are kept as well. The printer writes the
// space in a doc comment, so only a body or trailing comment is reported, and
// one that must keep another //word prefix needs an //ignore.
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

// bare are the words a tool reads with no colon, which the printer's rule does
// not cover: mksyscall reads //sys and //sysnb, and linters read //nolint.
var bare = map[string]struct{}{"nolint": {}, "sys": {}, "sysnb": {}}

// directive reports whether text, with its slashes removed, is a comment
// directive: the printer's rule, plus the bare words in bare. The rule is
// copied from go/ast's isDirective, which is not exported, so that this check
// and the formatter agree about which comments are prose.
func directive(text string) bool {
	// A directive's argument follows a space or a tab; mksyscall
	// writes a tab.
	fields := strings.Fields(text)
	if len(fields) > 0 {
		if _, ok := bare[fields[0]]; ok {
			return true
		}
	}
	for _, prefix := range []string{"line ", "extern ", "export "} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	colon := strings.Index(text, ":")
	if colon <= 0 || colon+1 >= len(text) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		b := text[i]
		if !('a' <= b && b <= 'z' || '0' <= b && b <= '9') {
			return false
		}
	}
	return true
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
	if directive(rest) {
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
