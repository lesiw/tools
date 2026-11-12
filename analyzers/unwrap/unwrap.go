// Package unwrap reports a file whose wrapped constructs fit on one line.
//
// The rules need no type information:
// a wrapped signature, call, binary expression, header or single-spec block
// goes back on one line when the line it produces fits. The width is
// linelen's, so a driver that sets none joins everything it can.
//
// An argument list the author broke into two or more indented lines may be a
// grouping, and nothing but the layout says so, so it is left alone.
package unwrap

import (
	"bytes"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports files the join rules would change.
var Analyzer = &analysis.Analyzer{
	Name: "unwrap",
	Doc:  "reports wrapped constructs that fit on one line",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out, err := applyAll(name, src)
		if err != nil || bytes.Equal(out, src) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     file.FileStart + token.Pos(firstDiff(src, out)),
			Message: "a wrapped construct fits on one line",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "Join the lines",
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

// firstDiff returns the offset of the first line that differs between the two
// sources, so a diagnostic points at the construct that changes.
func firstDiff(a, b []byte) (off int) {
	var (
		linesA = bytes.Split(a, []byte("\n"))
		linesB = bytes.Split(b, []byte("\n"))
	)
	for i := range linesA {
		if i >= len(linesB) || !bytes.Equal(linesA[i], linesB[i]) {
			return off
		}
		off += len(linesA[i]) + 1
	}
	return 0
}
