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
// heading, a bracketed name that resolves is a doc link, and a line beginning
// with a dash is a list item.
//
// A doc link may be followed by parentheses, and they are printed as written.
// A bracketed name that does not resolve, whose parentheses hold nothing but a
// target, is a Markdown link. A target holds no spaces or tabs, balances its
// unescaped parentheses, or runs in angle brackets. Either form may be empty.
package docsyntax

import (
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "docsyntax",
	Doc:  "reports markup syntax in Go comments",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	parser := newParser(pass)
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			check(pass, group, parser)
		}
	}
	return nil, nil
}

// newParser returns the parser godoc renders this package's doc comments with,
// so that a bracket it leaves unresolved is one godoc prints as literal text.
func newParser(pass *analysis.Pass) *comment.Parser {
	files := make(map[string]*ast.File, len(pass.Files))
	for _, file := range pass.Files {
		files[pass.Fset.File(file.Pos()).Name()] = file
	}
	// The analyzers alongside this one build SSA from these same syntax trees,
	// so this read must not change them. Without AllDecls, go/doc removes each
	// file's unexported declarations in place; without PreserveAST it drops
	// their comments and the bodies of their functions.
	//
	// doc.NewFromFiles sets each test file aside as a source of examples, and
	// a comment in one names declarations in that file. Keeping those files
	// means doc.New, whose *ast.Package argument is deprecated.
	pkg := doc.New(
		&ast.Package{Name: pass.Pkg.Name(), Files: files},
		pass.Pkg.Path(), doc.AllDecls|doc.PreserveAST,
	)
	return pkg.Parser()
}

// check reports the first markup construct a comment group carries. The
// group is one doc comment, so one finding is the whole of it.
func check(pass *analysis.Pass, group *ast.CommentGroup, parser *comment.Parser) {
	for _, c := range group.List {
		for line := range strings.SplitSeq(c.Text, "\n") {
			if text, ok := prose(line); ok {
				if construct := find(text, parser); construct != "" {
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
func find(text string, parser *comment.Parser) string {
	switch {
	case strings.HasPrefix(text, "```"):
		return "fenced code block not supported by Go doc syntax"
	case strings.Contains(text, "`"):
		return "code span not supported by Go doc syntax"
	case strings.Contains(text, "**"):
		return "bold not supported by Go doc syntax"
	case isLink(text, parser):
		return "markdown link not supported by Go doc syntax"
	}
	return ""
}

// isLink reports whether text holds a Markdown link. A bracketed name that
// resolves is a doc link, and godoc prints its parentheses as written. A
// parenthetical that holds no target is not a link: Markdown leaves it as
// written too, so the two readings agree.
func isLink(text string, parser *comment.Parser) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != '[' {
			continue
		}
		end := strings.IndexByte(text[i:], ']')
		if end < 0 {
			return false
		}
		name := text[i+1 : i+end]
		after := text[i+end+1:]
		if isTarget(after) && !resolves(name, parser) {
			return true
		}
		i += end
	}
	return false
}

// isTarget reports whether text begins with a parenthetical holding a link
// target and nothing else.
func isTarget(text string) bool {
	if !strings.HasPrefix(text, "(") {
		return false
	}
	rest := text[1:]
	if strings.HasPrefix(rest, "<") {
		for i := 1; i < len(rest); i++ {
			switch {
			case rest[i] == '\\' && i+1 < len(rest) && isPunct(rest[i+1]):
				i++
			case rest[i] == '>':
				return strings.HasPrefix(rest[i+1:], ")")
			}
		}
		return false
	}
	var depth int
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case ' ', '\t':
			return false
		case '\\':
			if i+1 < len(rest) && isPunct(rest[i+1]) {
				i++
			}
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return true
			}
			depth--
		}
	}
	return false
}

// isPunct reports whether c is ASCII punctuation, the only kind of character a
// backslash escapes in Markdown.
func isPunct(c byte) bool {
	switch {
	case '!' <= c && c <= '/', ':' <= c && c <= '@':
		return true
	case '[' <= c && c <= '`', '{' <= c && c <= '~':
		return true
	}
	return false
}

// resolves reports whether name, written inside brackets, is a doc link.
func resolves(name string, parser *comment.Parser) bool {
	// A line that begins with a bracket is always one paragraph.
	para := parser.Parse("[" + name + "]").Content[0].(*comment.Paragraph)
	for _, text := range para.Text {
		if _, ok := text.(*comment.DocLink); ok {
			return true
		}
	}
	return false
}
