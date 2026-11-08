// Package whitespace reports a file whose blank lines around declarations do
// not follow the layout rules.
//
// The rules are: one blank line between a multi-line top-level declaration and
// its neighbors, and a blank line before a doc comment inside a parenthesized
// block or a struct field list.
package whitespace

import (
	"bytes"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports files the spacing rules would change.
var Analyzer = &analysis.Analyzer{
	Name: "whitespace",
	Doc: "reports blank lines around declarations that do not follow the " +
		"layout rules",
	Run: run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out, err := space(name, src)
		if err != nil || bytes.Equal(out, src) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     file.FileStart + token.Pos(firstDiff(src, out)),
			Message: "a declaration needs a blank line above it",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "Set the blank lines",
				TextEdits: []analysis.TextEdit{{
					Pos:     file.FileStart,
					End:     file.FileStart + token.Pos(len(src)),
					NewText: out,
				}},
			}},
		})
	}
	return nil, nil
}
