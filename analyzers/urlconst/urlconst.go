// Package urlconst provides an analysis.Analyzer that reports a constant URL
// that url.Parse rejects.
//
// net/url.Parse is the parser for that format. It returns the same error for
// a rejected constant every time. Only the author knows which URL was meant,
// so the analyzer reports and offers no fix.
//
// Only a constant argument to a direct call is checked. A URL held in a
// variable or parameter is left alone, and so is a call through a function
// value. A deliberately malformed URL, as in an example that prints the
// error, is reported too.
//
// The rule is inspired by staticcheck's SA1007.
package urlconst

import (
	"go/ast"
	"go/constant"
	"net/url"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "urlconst",
	Doc:  "reports a constant URL that url.Parse rejects",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the URL argument of a url.Parse call when it is a constant
// that does not parse.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return
	}
	if fn.Pkg().Path() != "net/url" || fn.Name() != "Parse" {
		return
	}
	if fn.Signature().Recv() != nil {
		return
	}
	if len(call.Args) == 0 {
		return
	}
	arg := call.Args[0]
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return
	}
	if _, err := url.Parse(constant.StringVal(tv.Value)); err != nil {
		reason := err.Error()
		// A *url.Error repeats the whole URL, which a constant may hold
		// across lines; the cause is the part that helps.
		if urlErr, ok := err.(*url.Error); ok && urlErr.Err != nil {
			reason = urlErr.Err.Error()
		}
		pass.Reportf(arg.Pos(), "URL does not parse: %s", reason)
	}
}
