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

	"lesiw.io/tools/internal/source"
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
			start := source.Start(src, pass.Fset.Position(gen.Pos()).Offset)
			end := source.End(src, pass.Fset.Position(gen.End()).Offset)
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
