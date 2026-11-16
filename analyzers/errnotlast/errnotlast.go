// Package errnotlast provides an analysis.Analyzer that reports a function
// whose last error result is followed by another result.
//
// The Google Go Style Guide says: "By convention, error is the last result
// parameter." The error goes last, so a caller writes value, err := fetch().
//
// A result is an error result when its type is the predeclared error, or a
// named type that implements error and ends in Error. The suffix is how a
// reader recognizes an error type. An alias is resolved before the suffix
// test. For a pointer, the suffix test uses the name of the type it points to,
// and the pointer is what must implement error. A type parameter is left alone
// as an earlier result, whatever its constraint. An unnamed type does not have
// a name for the suffix test. A final result that implements error is
// accepted, whatever its name.
//
// The check covers the signature of a function declaration, a method
// declaration or a function literal. Every other function type is left to
// errorlast. Both analyzers report a declaration that names a function type
// and contains a function literal.
//
// Reordering results changes every call site, so the analyzer reports only.
// The rule is adapted from revive's error-return, which checks only function
// declarations and matches only the predeclared error type.
package errnotlast

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/errorresult"
)

var Analyzer = &analysis.Analyzer{
	Name: "errnotlast",
	Doc:  "reports a function whose last error result is followed by another result",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				errorresult.Check(pass, n.Type)
			case *ast.FuncLit:
				errorresult.Check(pass, n.Type)
			}
			return true
		})
	}
	return nil, nil
}
