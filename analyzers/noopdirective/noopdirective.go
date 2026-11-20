// Package noopdirective provides an analysis.Analyzer that reports a comment
// that reads as a //go: directive but for whitespace.
//
// cmd/compile's "Compiler Directives" section says each directive "must be
// placed its own line, with only leading spaces and tabs allowed before the
// comment, and there must be no space between the comment opening and the name
// of the directive, to distinguish it from a regular comment"
// (https://pkg.go.dev/cmd/compile#hdr-Compiler_Directives). A line that
// separates // from go: with whitespace is an ordinary comment, so a directive
// written that way does nothing: a // go:build line constrains nothing, and a
// // go:embed line embeds nothing.
//
// The rule reads a comment before the package clause and a comment that
// documents a func, var, const, or type declaration or spec, which gofmt
// leaves indented inside a var, const, or type block.
//
// The diagnostic carries no fix. Removing the whitespace makes the directive
// take effect, and the effect is not always the one the author wanted: a
// // go:build line may exclude the file it heads, and a // go:embed line fails
// to compile when its pattern matches nothing.
//
// The README catalogs every check and exemption.
//
// The rule is adapted from staticcheck's SA9009.
package noopdirective

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "noopdirective",
	Doc:  "reports a comment that reads as a //go: directive but for whitespace",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		documented := documented(file)
		for _, group := range file.Comments {
			// The go command reads a build constraint from the comment
			// group before the package clause, indented or not.
			header := group.End() < file.Package
			for _, comment := range group.List {
				if !header && !documented[comment] {
					continue
				}
				if name, ok := directive(comment); ok {
					pass.Report(analysis.Diagnostic{
						Pos: comment.Pos(),
						End: comment.End(),
						Message: "whitespace after // keeps go:" + name +
							" from being read as a directive",
					})
				}
			}
		}
	}
	return nil, nil
}

// documented returns the comments in a doc comment of a func, var, const, or
// type declaration or spec, which is where a directive above a declaration is
// read.
func documented(file *ast.File) map[*ast.Comment]bool {
	docs := make(map[*ast.Comment]bool)
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			collect(docs, decl.Doc)
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				continue
			}
			collect(docs, decl.Doc)
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					collect(docs, spec.Doc)
				case *ast.ValueSpec:
					collect(docs, spec.Doc)
				}
			}
		}
	}
	return docs
}

func collect(docs map[*ast.Comment]bool, group *ast.CommentGroup) {
	if group == nil {
		return
	}
	for _, comment := range group.List {
		docs[comment] = true
	}
}

// directive returns the name in a comment that was written as a directive and
// broken by whitespace after //, and returns false for any other comment.
func directive(comment *ast.Comment) (string, bool) {
	text, ok := strings.CutPrefix(comment.Text, "//")
	if !ok {
		return "", false
	}
	trimmed := strings.TrimLeft(text, " \t")
	if trimmed == text {
		return "", false
	}
	tail, ok := strings.CutPrefix(trimmed, "go:")
	if !ok || tail == "" || tail[0] < 'a' || tail[0] > 'z' {
		return "", false
	}
	// The name stops at the first rune a name cannot hold, so a sentence
	// continuing the line is not quoted in the message.
	for i, r := range tail {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return tail[:i], true
		}
	}
	return tail, true
}
