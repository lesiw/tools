// Package docstub provides an analysis.Analyzer that reports a doc comment
// that only repeats the declaration's name.
//
// Go's page on doc comments (https://go.dev/doc/comment) asks a function's
// doc comment to explain what it returns, or what it does when it is called
// for its side effects, and asks the same of a method. It asks a type's doc
// comment to explain what each instance represents or provides. "Foo does
// foo." says nothing the declaration does not already say.
//
// The rule is adapted from go-critic's docStub, which reports an exported
// declaration documented as its name and a placeholder ("Foo ...", "Foo xxx").
//
// The fix removes the comment's lines. It is withheld when another comment
// ends on the comment's first line, since deleting the line would take the
// closer with it.
package docstub

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "docstub",
	Doc:  "reports a doc comment that only repeats the declaration's name",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tokFile := pass.Fset.File(file.FileStart)
		src, err := pass.ReadFile(tokFile.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", tokFile.Name(), err)
		}
		for _, d := range documented(file) {
			if !repeats(d.comment, d.name.Name) || hasDirective(d.comment) {
				continue
			}
			diagnostic := analysis.Diagnostic{
				Pos:     d.name.Pos(),
				Message: "doc comment only repeats the name " + d.name.Name,
			}
			if edit, ok := removal(tokFile, src, d.comment); ok {
				diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
					Message:   "Remove the comment",
					TextEdits: []analysis.TextEdit{edit},
				}}
			}
			pass.Report(diagnostic)
		}
	}
	return nil, nil
}

// declaration is a declared name and the comment above it.
type declaration struct {
	name    *ast.Ident
	comment *ast.CommentGroup
}

// documented returns each declaration in file that carries a doc comment, and
// the methods of the interfaces among them. A value spec that names more than
// one value has no single name, so it is left out.
func documented(file *ast.File) (decls []declaration) {
	add := func(name *ast.Ident, comment *ast.CommentGroup) {
		if comment != nil {
			decls = append(decls, declaration{name, comment})
		}
	}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			add(decl.Name, decl.Doc)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					add(spec.Name, specDoc(decl, spec.Doc))
					iface, ok := spec.Type.(*ast.InterfaceType)
					if !ok {
						continue
					}
					for _, method := range iface.Methods.List {
						if len(method.Names) == 0 {
							continue // an embedded interface
						}
						add(method.Names[0], method.Doc)
					}
				case *ast.ValueSpec:
					if len(spec.Names) == 1 {
						add(spec.Names[0], specDoc(decl, spec.Doc))
					}
				}
			}
		}
	}
	return decls
}

// specDoc returns the comment that documents a spec: its own, or the
// declaration's when the declaration holds one spec.
func specDoc(decl *ast.GenDecl, own *ast.CommentGroup) *ast.CommentGroup {
	if own == nil && len(decl.Specs) == 1 {
		return decl.Doc
	}
	return own
}

// repeats reports whether comment restates name and says nothing more.
func repeats(comment *ast.CommentGroup, name string) bool {
	if !plain(comment) {
		return false
	}
	var (
		words = splitWords(commentText(comment))
		own   = splitWords(name)
	)
	// Filler words may introduce the name. The name is tried first, so a
	// name that is also a filler word (Return) still matches.
	for len(words) > 0 && !starts(words, own) && filler(words[0]) {
		words = words[1:]
	}
	if !starts(words, own) {
		return false
	}
	// The words after the name are the name again, or they are nothing. A
	// comment that names part of the name is kept, as in "A SplitKey is a
	// key".
	rest := slices.DeleteFunc(words[len(own):], filler)
	return len(rest) == 0 || slices.Equal(rest, own) || agrees(rest, own)
}

// starts reports whether words begins with own. An empty own begins nothing.
func starts(words, own []string) bool {
	return len(own) > 0 && len(words) >= len(own) &&
		slices.Equal(words[:len(own)], own)
}

// agrees reports whether rest is own with its last word inflected, as "Fill
// fills." and "Copy copies." repeat their names.
func agrees(rest, own []string) bool {
	if len(rest) != len(own) || len(own) == 0 {
		return false
	}
	if !slices.Equal(rest[:len(own)-1], own[:len(own)-1]) {
		return false
	}
	var (
		last = rest[len(rest)-1]
		base = own[len(own)-1]
	)
	if last == base+"s" || last == base+"es" {
		return true
	}
	return strings.HasSuffix(base, "y") && last == base[:len(base)-1]+"ies"
}

// filler reports whether word carries nothing of its own.
func filler(word string) bool {
	_, ok := fillers[word]
	return ok
}

// fillers are the words that carry nothing of their own: the articles, the
// forms of be and the verbs an author writes in place of a description, and
// the placeholders written when only a linter wants a comment.
var fillers = map[string]struct{}{
	"a": {}, "an": {}, "the": {},
	"is": {}, "are": {},
	"does": {}, "returns": {}, "represents": {},
	"creates": {}, "constructs": {},
	"whatever": {}, "xxx": {},
}

// plain reports whether the comment is prose about the declaration. A mark is
// any character other than a letter, a digit, or a space, and one that is not
// at the end of the text says something the words do not: "- " lists an item,
// "Header.Type" names a field, and "Read -> Write" names an order.
func plain(comment *ast.CommentGroup) bool {
	for _, r := range strings.TrimRightFunc(commentText(comment), notWord) {
		if notWord(r) && !unicode.IsSpace(r) {
			return false // a mark
		}
	}
	return true
}

// hasDirective reports whether a line of the comment is a directive, or a line
// whose slashes have no space after them: either belongs to another tool or is
// a formatting fault that commentfmt reports, and deleting the comment would
// delete the line with it.
func hasDirective(comment *ast.CommentGroup) bool {
	for _, c := range comment.List {
		rest, ok := strings.CutPrefix(c.Text, "//")
		if ok && rest != "" && rest[0] != ' ' {
			return true
		}
	}
	return false
}

// notWord reports whether r is not part of a word.
func notWord(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// commentText returns the text of a comment group with the markers removed. A
// block comment decorates each line with a star; a line comment does not.
func commentText(comment *ast.CommentGroup) string {
	var b strings.Builder
	for _, c := range comment.List {
		var (
			block = strings.HasPrefix(c.Text, "/*")
			text  = unmark(c.Text)
		)
		for line := range strings.SplitSeq(text, "\n") {
			line = strings.TrimLeft(line, " \t")
			if block {
				line = strings.TrimPrefix(line, "*")
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// unmark returns a comment's text without the markers that delimit it.
func unmark(text string) string {
	if rest, ok := strings.CutPrefix(text, "//"); ok {
		return rest
	}
	return strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
}

// splitWords returns the words of s, lowercased, with punctuation and camel
// case taken apart, so that a name and the comment repeating it split the same
// way. Digits stay with the word they follow, so UTF8 and utf8 agree.
func splitWords(s string) []string {
	var (
		words []string
		word  []rune
		runes = []rune(s)
		flush = func() {
			if len(word) > 0 {
				words = append(words, strings.ToLower(string(word)))
				word = word[:0]
			}
		}
	)
	for i, r := range runes {
		if notWord(r) {
			flush()
			continue
		}
		if len(word) > 0 && unicode.IsUpper(r) {
			prev, next := runes[i-1], rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			// A run of capitals is one word up to its last letter.
			if !unicode.IsUpper(prev) || unicode.IsLower(next) {
				flush()
			}
		}
		word = append(word, r)
	}
	flush()
	return words
}

// removal returns the edit that deletes the comment's lines, and reports
// whether the comment can be deleted whole. The lines are the file's own:
// LineStart is indexed by them, and PositionFor with the adjustment off
// returns them after a //line directive too.
func removal(tokFile *token.File, src []byte, comment *ast.CommentGroup) (analysis.TextEdit, bool) {
	// The file may have changed since it was parsed, and then every offset
	// below points into the wrong text.
	if tokFile.Size() != len(src) {
		return analysis.TextEdit{}, false
	}
	var (
		first = tokFile.PositionFor(comment.Pos(), false).Line
		last  = tokFile.PositionFor(comment.End(), false).Line
		start = tokFile.LineStart(first)
		lead  = src[tokFile.Offset(start):tokFile.Offset(comment.Pos())]
	)
	if len(bytes.TrimLeft(lead, " \t")) != 0 {
		return analysis.TextEdit{}, false
	}
	return analysis.TextEdit{
		Pos: start,
		End: tokFile.LineStart(last + 1),
	}, true
}
