// Package queryadd provides an analysis.Analyzer that reports an Add call on
// the values returned by URL.Query.
//
// The net/url documentation says that Query parses RawQuery and returns the
// corresponding values, and that Query silently discards malformed value
// pairs. Query builds a separate map, so a change to it does not affect
// RawQuery. To change the URL, assign the encoded values to RawQuery:
//
//	q := u.Query()
//	q.Add("some-key", "some-value")
//	u.RawQuery = q.Encode()
//
// Encoding the map back into RawQuery drops the malformed pairs that Query
// discarded.
//
// The analyzer does not report an Add call on a variable or on the result of a
// Query method other than (*url.URL).Query. The README catalogs every check
// and exemption.
//
// The rule is inspired by staticcheck's SA4027, which also reports Set and
// Del.
//
// Report only: the two repairs are to delete the call and to declare a
// variable for the values, call Add on it, and assign the result of Encode()
// to RawQuery. Deleting the call can drop the evaluation of the receiver and
// arguments, and the line may carry a comment. The encode fix makes the add
// take effect, which changes behavior.
package queryadd

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "queryadd",
	Doc:  "reports an Add call on the values returned by URL.Query",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !addsToQuery(pass, call) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "the added value is discarded",
			})
			return true
		})
	}
	return nil, nil
}

// addsToQuery reports whether call is an Add call on the values returned by
// URL.Query.
func addsToQuery(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Add" {
		return false
	}
	if pass.TypesInfo.Types[sel.X].IsType() {
		// url.Values.Add(values, key, value)
		return len(call.Args) > 0 && isQueryCall(pass, call.Args[0])
	}
	recv, ok := ast.Unparen(sel.X).(*ast.CallExpr)
	if !ok {
		return false
	}
	if isQuery(pass, recv) {
		return true
	}
	// url.Values(u.Query()).Add(key, value)
	return isValuesConversion(pass, recv) && isQueryCall(pass, recv.Args[0])
}

// isQueryCall reports whether e is a call to the Query method of net/url.URL.
func isQueryCall(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	return ok && isQuery(pass, call)
}

// isQuery reports whether call is a call to the Query method of net/url.URL.
func isQuery(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.FullName() == "(*net/url.URL).Query"
}

// isValuesConversion reports whether call converts a single value to
// net/url.Values.
func isValuesConversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	tv, ok := pass.TypesInfo.Types[ast.Unparen(call.Fun)]
	if !ok || !tv.IsType() {
		return false
	}
	named, ok := types.Unalias(pass.TypesInfo.TypeOf(call)).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "net/url" &&
		obj.Name() == "Values"
}
