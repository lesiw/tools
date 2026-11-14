// Package errorlast provides an analysis.Analyzer that reports a signature
// whose error result is not the last one.
//
// The Google Go Style Guide states: "By convention, error is the last result
// parameter." A caller writes the error last: n, err := lookup().
//
// A result is an error result when its type is the predeclared error, or a
// type that implements error under a name ending in Error. The suffix is how a
// reader recognizes an error type; errname asks error types to carry it.
package errorlast

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errorlast",
	Doc:  "reports a signature whose error result is not the last one",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncType); ok {
				check(pass, fn)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the first error result of fn when a result follows it.
func check(pass *analysis.Pass, fn *ast.FuncType) {
	if fn.Results == nil || len(fn.Results.List) == 0 {
		return
	}
	results := fn.Results.List
	if isError(pass.TypesInfo.TypeOf(results[len(results)-1].Type)) {
		return
	}
	for _, field := range results {
		if !isError(pass.TypesInfo.TypeOf(field.Type)) {
			continue
		}
		pass.Reportf(
			field.Type.Pos(), "%s is not the last result",
			types.ExprString(field.Type),
		)
		return
	}
}

// isError reports whether t is the type of an error result.
func isError(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	// Implements says nothing about an uninstantiated type parameter.
	if _, ok := t.(*types.TypeParam); ok {
		return false
	}
	errorType := types.Universe.Lookup("error").Type()
	// The predeclared error is spelled in lowercase, so the name test below
	// would miss it.
	if types.Identical(t, errorType) {
		return true
	}
	err, ok := errorType.Underlying().(*types.Interface)
	if !ok || !types.Implements(t, err) {
		return false
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	n, ok := t.(*types.Named)
	return ok && strings.HasSuffix(n.Obj().Name(), "Error")
}
