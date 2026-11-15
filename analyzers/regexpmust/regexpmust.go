// Package regexpmust provides an analysis.Analyzer that reports a
// regexp.Compile of a constant pattern, whose error is always nil.
//
// The pattern never changes, so the error is always nil and any check for it
// is dead code. regexp.MustCompile compiles the same pattern and panics where
// Compile returns an error.
//
// The analyzer reports and offers no fix.
//
// A pattern that does not compile is a different fault, reported by
// regexpconst: the swap would turn the returned error into a panic without
// fixing the pattern.
//
// The rule is adapted from go-critic's regexpMust.
package regexpmust

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "regexpmust",
	Doc:  "reports a regexp.Compile of a constant pattern, whose error is always nil",
	Run:  run,
}

// compilers are the functions the rule covers, each with the compiler for its
// pattern and the name of the function that panics where it returns an error.
var compilers = map[string]struct {
	compile func(string) (*regexp.Regexp, error)
	must    string
}{
	"Compile":      {regexp.Compile, "MustCompile"},
	"CompilePOSIX": {regexp.CompilePOSIX, "MustCompilePOSIX"},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			must, ok := mustFor(pass, call)
			if !ok {
				return true
			}
			pass.Reportf(call.Pos(), "the error of a constant pattern is always nil; use %s", must)
			return true
		})
	}
	return nil, nil
}

// mustFor returns the Must name for a regexp compile whose constant pattern
// compiles, and false for any other call.
func mustFor(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	form, ok := compilers[sel.Sel.Name]
	if !ok {
		return "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "regexp" {
		return "", false
	}
	if len(call.Args) == 0 {
		return "", false
	}
	tv, ok := pass.TypesInfo.Types[call.Args[0]]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	if _, err := form.compile(constant.StringVal(tv.Value)); err != nil {
		return "", false
	}
	return form.must, true
}
