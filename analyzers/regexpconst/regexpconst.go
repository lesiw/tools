// Package regexpconst provides an analysis.Analyzer that reports a constant
// regexp pattern that does not compile.
//
// The pattern never changes, so the call fails the same way every time:
// MustCompile and MustCompilePOSIX panic, and every other function returns
// the error. Only the author knows which pattern was meant, so the analyzer
// reports and offers no fix.
//
// Only a constant argument to a package-qualified call is checked. A pattern
// that is deliberately invalid, as in an example that prints the parse error,
// is reported too.
//
// The rule is adapted from staticcheck's SA1000, which does not check the
// POSIX functions.
package regexpconst

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"
	"regexp/syntax"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "regexpconst",
	Doc:  "reports a constant regexp pattern that does not compile",
	Run:  run,
}

// The value compiles the pattern argument: the POSIX pair rejects an escape
// the Perl syntax accepts.
var compilers = map[string]func(string) (*regexp.Regexp, error){
	"Compile":          regexp.Compile,
	"CompilePOSIX":     regexp.CompilePOSIX,
	"MustCompile":      regexp.Compile,
	"MustCompilePOSIX": regexp.CompilePOSIX,
	"Match":            regexp.Compile,
	"MatchReader":      regexp.Compile,
	"MatchString":      regexp.Compile,
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

func check(pass *analysis.Pass, call *ast.CallExpr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	compile, ok := compilers[sel.Sel.Name]
	if !ok {
		return
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	// A Regexp value has methods with these names, and their argument is text
	// to match rather than a pattern.
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "regexp" {
		return
	}
	if len(call.Args) == 0 {
		return
	}
	pattern := call.Args[0]
	tv, ok := pass.TypesInfo.Types[pattern]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return
	}
	_, err := compile(constant.StringVal(tv.Value))
	syntaxErr, ok := err.(*syntax.Error)
	if !ok {
		return
	}
	// The code names the fault without quoting a pattern that may span lines.
	pass.Reportf(pattern.Pos(), "pattern does not compile: %s", syntaxErr.Code)
}
