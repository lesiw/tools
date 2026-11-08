// Package docsyntax provides an analysis.Analyzer that reports markup syntax
// that Go doc comments do not implement.
//
// Go has a doc comment syntax of its own, defined by go/doc/comment and
// described at https://go.dev/doc/comment. Markdown is a different language,
// and the parts of it that Go does not implement render as literal
// punctuation: a code span keeps its backticks, and a fenced block keeps its
// fence.
//
// Go doc syntax has forms of its own, and they are not findings: an indented
// block is a code block, a line beginning with a number sign and a space is a
// heading, a bracketed name with no URL is a doc link, and a line beginning
// with a dash is a list item.
package docsyntax

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "docsyntax",
	Doc:  "reports markup syntax in Go comments",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			check(pass, group)
		}
	}
	return nil, nil
}

// check reports the first markup construct a comment group carries. The
// group is one doc comment, so one finding is the whole of it.
func check(pass *analysis.Pass, group *ast.CommentGroup) {
	for _, comment := range group.List {
		for line := range strings.SplitSeq(comment.Text, "\n") {
			if text, ok := prose(line); ok {
				if construct := find(text); construct != "" {
					pass.Reportf(group.Pos(), "%s", construct)
					return
				}
			}
		}
	}
}

// prose returns the words of a comment line, and whether the line is prose at
// all. An indented line is a code block to godoc, and a code block is where a
// backtick belongs.
func prose(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "//"):
		line = strings.TrimPrefix(line, "//")
	case strings.HasPrefix(line, "/*"):
		line = strings.TrimPrefix(line, "/*")
		line = strings.TrimSuffix(line, "*/")
	case strings.HasPrefix(line, "*"):
		// A continuation line of a block comment.
		line = strings.TrimPrefix(line, "*")
	default:
		return "", false
	}
	if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
		return "", false
	}
	return strings.TrimSpace(line), true
}

// find returns the first markup construct in text that Go doc syntax does not
// support, or "".
//
// A heading is not one: go/doc/comment reads a line beginning with a number
// sign and a space as a heading. A fenced code block is one: the parser leaves
// the fence in the text, where a reader sees three backticks. A code block in
// Go doc syntax is an indented span of lines.
func find(text string) string {
	switch {
	case strings.HasPrefix(text, "```"):
		return "fenced code block not supported by Go doc syntax"
	case strings.Contains(text, "`"):
		return "code span not supported by Go doc syntax"
	case strings.Contains(text, "**"):
		return "bold not supported by Go doc syntax"
	case isLink(text):
		return "markdown link not supported by Go doc syntax"
	}
	return ""
}

// isLink reports whether text holds a Markdown link. A bracketed name with no
// URL after it is a doc link, which is godoc's own form.
func isLink(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != '[' {
			continue
		}
		end := strings.IndexByte(text[i:], ']')
		if end < 0 {
			return false
		}
		rest := text[i+end+1:]
		if strings.HasPrefix(rest, "(") {
			return true
		}
		i += end
	}
	return false
}
