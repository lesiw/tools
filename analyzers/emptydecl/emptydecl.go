// Package emptydecl provides an analysis.Analyzer that reports an empty
// declaration block.
//
// var (), const () and type () declare nothing, so they are a block that was
// emptied and left behind. Like go-critic's emptyDecl.
package emptydecl

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptydecl",
	Doc:  "reports an empty declaration block",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || len(gen.Specs) != 0 {
				continue
			}
			start := lineStart(src, pass.Fset.Position(gen.Pos()).Offset)
			end := lineEnd(src, pass.Fset.Position(gen.End()).Offset)
			pass.Report(analysis.Diagnostic{
				Pos:     gen.Pos(),
				End:     gen.End(),
				Message: "empty " + gen.Tok.String() + " block",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Remove the block",
					TextEdits: []analysis.TextEdit{{
						Pos: file.FileStart + token.Pos(start),
						End: file.FileStart + token.Pos(end),
					}},
				}},
			})
		}
	}
	return nil, nil
}

// lineStart returns the offset of the line start at or before off.
func lineStart(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}

// lineEnd returns the offset just past the newline that ends the line at off.
func lineEnd(src []byte, off int) int {
	for off < len(src) && src[off] != '\n' {
		off++
	}
	if off < len(src) {
		off++
	}
	return off
}
