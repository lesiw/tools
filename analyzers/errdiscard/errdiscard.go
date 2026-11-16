// Package errdiscard provides an analysis.Analyzer that reports a call whose
// only result is an error, written as a statement.
//
//	func remove(path string) {
//		os.Remove(path)
//	}
//
// Go's Code Review Comments say: "Do not discard errors using _ variables. If
// a function returns an error, check it to make sure the function succeeded.
// Handle the error, return it, or, in truly exceptional situations, panic."
//
// The error is the only result, so a statement that calls the function
// discards the only report of failure. The statement is reported when the call
// returns exactly one value and that value's type implements error.
//
// An assignment to the blank identifier is left alone, which the Code Review
// Comments would not allow: the assignment states that the discard is
// deliberate.
//
// A standard library method documented as never returning an error is left
// alone too, since discarding the error loses nothing. The README catalogs
// every check and exemption.
//
// Only an expression statement is reported, so a call under defer or go is
// left alone.
//
// The rule is inspired by gosec's G104. The suite's errcheck reports a call
// whose result holds an error, and reads the type only when it is a named
// type, a pointer, or a tuple, so a type parameter, an alias of error, and an
// unnamed interface are the type shapes this rule adds. discardedpure reports
// a pure callee whose result is dropped, so one statement can carry more than
// one message.
//
// Report only: the repair is the handling, which changes behavior and is the
// caller's code to write. A report the author means to keep is suppressed with
// an //ignore directive.
package errdiscard

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "errdiscard",
	Doc:  "reports a call whose only result is an error, written as a statement",
	Run:  run,
}

// neverFails holds the standard library methods documented as never returning
// an error. The receiver is part of the key because a method of the same name
// on another type can fail.
var neverFails = map[string]struct{}{
	"(*bytes.Buffer).WriteByte":       {},
	"(*hash/maphash.Hash).WriteByte":  {},
	"(*io.PipeReader).CloseWithError": {},
	"(*io.PipeWriter).CloseWithError": {},
	"(*strings.Builder).WriteByte":    {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.ExprStmt); ok {
				check(pass, stmt)
			}
			return true
		})
	}
	return nil, nil
}

// check reports stmt when the call in it has an error as its only result.
func check(pass *analysis.Pass, stmt *ast.ExprStmt) {
	call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
	if !ok || !onlyError(pass, call) {
		return
	}
	if _, fn, ok := uses.CalleeOf(pass, call.Fun); ok {
		if _, ok := neverFails[fn.FullName()]; ok {
			return
		}
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: calleeName(call.Fun) + " discards its error",
	})
}

// onlyError reports whether call's type implements error. A call with several
// results has a tuple type, which implements nothing. A type parameter's
// underlying type is its constraint interface, so go/types answers through the
// methods the constraint declares.
func onlyError(pass *analysis.Pass, call *ast.CallExpr) bool {
	t := pass.TypesInfo.TypeOf(call)
	if t == nil {
		return false
	}
	err := types.Universe.Lookup("error").Type()
	return types.Implements(t, err.Underlying().(*types.Interface))
}

// calleeName returns the callee as it is written, or the call when the callee
// has no name, as with a function literal.
func calleeName(fun ast.Expr) string {
	switch fun := ast.Unparen(fun).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return types.ExprString(fun)
	}
	return "the call"
}
