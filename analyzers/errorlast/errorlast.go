// Package errorlast provides an analysis.Analyzer that reports a function
// type whose last error result is followed by another result.
//
// The Google Go Style Guide says: "By convention, error is the last result
// parameter." The error goes last, so a caller writes value, err := fetch().
//
// A result is an error result when its type is the predeclared error, or a
// type that implements error under a name ending in Error. The suffix is how a
// reader recognizes an error type; errname reports an error type without it.
// An alias is resolved before the suffix test. For a pointer, the suffix test
// uses the name of the type it points to, and the pointer is what must
// implement error. A type parameter is left alone as an earlier result,
// whatever its constraint. A final result that implements error is accepted,
// whatever its name.
//
// The check covers every function type that is not the signature of a
// function declaration, a method declaration or a function literal. Those
// signatures are left to errnotlast. Both analyzers report a declaration that
// names a function type and contains a function literal.
//
// Reordering results changes every call site, so the analyzer reports only.
// The rule is adapted from revive's error-return, which checks only function
// declarations and matches only the predeclared error type. That check is
// split: errnotlast reports the declarations and literals, and this rule
// reports the function types.
package errorlast

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/errorresult"
)

var Analyzer = &analysis.Analyzer{
	Name: "errorlast",
	Doc:  "reports a function type whose last error result is followed by another result",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// errnotlast checks a function's type, so this pass skips it.
		functions := make(map[*ast.FuncType]struct{})
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				functions[n.Type] = struct{}{}
			case *ast.FuncLit:
				functions[n.Type] = struct{}{}
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncType); ok {
				if _, isFunction := functions[fn]; !isFunction {
					errorresult.Check(pass, fn)
				}
			}
			return true
		})
	}
	return nil, nil
}
