// Package dotimports provides an analysis.Analyzer that reports a dot import.
//
// The Import declarations section of the spec says: "If an explicit period
// (.) appears instead of a name, all the package's exported identifiers
// declared in that package's package block will be declared in the importing
// source file's file block and must be accessed without a qualifier"
// (https://go.dev/ref/spec#Import_declarations). A name from the imported
// package is declared in the file block, and a reader has to find the import
// to learn which package supplied it.
//
// Go Code Review Comments allows the form in a test file that cannot be part
// of the package under test because of a circular dependency
// (https://go.dev/wiki/CodeReviewComments#import-dot). The rule reports such a
// file too, because its names still have no qualifier.
//
// The rule is adapted from revive's dot-imports.
package dotimports

import "golang.org/x/tools/go/analysis"

var Analyzer = &analysis.Analyzer{
	Name: "dotimports",
	Doc:  "reports a dot import",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			if spec.Name == nil || spec.Name.Name != "." {
				continue
			}
			pass.Reportf(spec.Pos(), "dot import of %s", spec.Path.Value)
		}
	}
	return nil, nil
}
