// Package deferchain provides an analysis.Analyzer that reports a deferred
// call that returns a function.
//
// The spec's Defer statements section says "Each time a "defer" statement
// executes, the function value and parameters to the call are evaluated as
// usual and saved anew but the actual function is not invoked."
// (https://go.dev/ref/spec#Defer_statements). In
//
//	defer cleanup()
//
// where cleanup returns a func(), the call to cleanup runs at the surrounding
// function's return and the returned function is discarded. The usual intent
// is to call the returned function:
//
//	defer cleanup()()
//
// A call that returns several results is left alone: its result is a tuple.
// A multiple-value expression cannot be the callee of another call. The fix
// appends calls through the returned functions while each can be called with
// no arguments. If the fix reaches a result type it has already seen, the
// report has no fix. When the first returned function cannot be called with no
// arguments, the report has no fix, because no value is available to pass. The
// fix stops at a later function that cannot be called with no arguments, and
// the next run reports the remaining deferred call without a fix.
//
// The rule is adapted from staticcheck's SA9010.
package deferchain

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "deferchain",
	Doc:  "reports a deferred call that returns a function",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.DeferStmt)
			if !ok {
				return true
			}
			typ := pass.TypesInfo.TypeOf(stmt.Call)
			if typ == nil {
				return true
			}
			if _, ok := typ.Underlying().(*types.Signature); !ok {
				return true
			}
			d := analysis.Diagnostic{
				Pos:     stmt.Pos(),
				End:     stmt.End(),
				Message: "deferred call returns a function that is not called",
			}
			if calls, ok := fixCalls(typ); ok {
				d.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Call the returned functions in the defer",
					TextEdits: []analysis.TextEdit{{
						Pos:     stmt.Call.End(),
						End:     stmt.Call.End(),
						NewText: []byte(calls),
					}},
				}}
			}
			pass.Report(d)
			return true
		})
	}
	return nil, nil
}

// fixCalls returns the calls to append to a defer whose call returns typ. It
// stops at a result that is not a function or at a returned function that
// cannot be called with no arguments, and it returns false for a result type
// that repeats.
func fixCalls(typ types.Type) (string, bool) {
	var (
		seen  []types.Type
		calls int
	)
	for {
		for _, prev := range seen {
			if types.Identical(prev, typ) {
				return "", false
			}
		}
		sig, ok := typ.Underlying().(*types.Signature)
		if !ok {
			return strings.Repeat("()", calls), true
		}
		if !noArgs(sig) {
			return strings.Repeat("()", calls), calls > 0
		}
		seen = append(seen, typ)
		calls++
		results := sig.Results()
		if results.Len() != 1 {
			return strings.Repeat("()", calls), true
		}
		typ = results.At(0).Type()
	}
}

// noArgs reports whether a call to sig can pass no arguments.
func noArgs(sig *types.Signature) bool {
	params := sig.Params().Len()
	return params == 0 || sig.Variadic() && params == 1
}
