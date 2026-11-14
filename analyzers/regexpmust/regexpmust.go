// Package regexpmust provides an analysis.Analyzer that reports a
// regexp.Compile of a constant pattern, whose error is always nil.
//
// The pattern never changes, so the error is always nil and any check for it
// is dead code. regexp.MustCompile compiles the same pattern and panics where
// Compile returns an error.
//
// Where the pattern is bound and the second result is blank, the fix swaps in
// the Must form and drops the blank. Where the caller reads the error, the
// repair deletes the branch that reads it, so the analyzer reports without a
// fix. It also reports without a fix when both results are discarded and when
// a comment sits between the two targets.
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
	"go/token"
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
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, stack, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports one call whose constant pattern compiles, with the edit that
// swaps in the Must form when the caller throws the error away.
func check(pass *analysis.Pass, file *ast.File, stack []ast.Node, call *ast.CallExpr) {
	must, sel, ok := mustFor(pass, call)
	if !ok {
		return
	}
	d := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "the error of a constant pattern is always nil; use " + must,
	}
	if kept, blank := targets(file, stack, call); kept != nil {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace with " + must,
			TextEdits: []analysis.TextEdit{
				{Pos: kept.End(), End: blank.End()},
				{Pos: sel.Pos(), End: sel.End(), NewText: []byte(must)},
			},
		}}
	}
	pass.Report(d)
}

// mustFor returns the Must name and the identifier to replace for a regexp
// compile whose constant pattern compiles, and false for any other call.
func mustFor(pass *analysis.Pass, call *ast.CallExpr) (string, *ast.Ident, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", nil, false
	}
	form, ok := compilers[sel.Sel.Name]
	if !ok {
		return "", nil, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", nil, false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "regexp" {
		return "", nil, false
	}
	if len(call.Args) == 0 {
		return "", nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Args[0]]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", nil, false
	}
	if _, err := form.compile(constant.StringVal(tv.Value)); err != nil {
		return "", nil, false
	}
	return form.must, sel.Sel, true
}

// targets returns the two assignment targets that hold call as the only value,
// when the second is the blank identifier and no comment sits between them.
func targets(file *ast.File, stack []ast.Node, call *ast.CallExpr) (kept, blank ast.Expr) {
	var lhs, rhs []ast.Expr
	switch parent := holder(stack).(type) {
	case *ast.AssignStmt:
		lhs, rhs = parent.Lhs, parent.Rhs
	case *ast.ValueSpec:
		rhs = parent.Values
		for _, name := range parent.Names {
			lhs = append(lhs, name)
		}
	}
	if len(lhs) != 2 || len(rhs) != 1 {
		return nil, nil
	}
	if ast.Unparen(rhs[0]) != call {
		return nil, nil
	}
	second, ok := lhs[1].(*ast.Ident)
	if !ok || second.Name != "_" {
		return nil, nil
	}
	if first, ok := lhs[0].(*ast.Ident); ok && first.Name == "_" {
		return nil, nil
	}
	if commented(file, lhs[0].End(), second.End()) {
		return nil, nil
	}
	return lhs[0], second
}

// holder returns the statement or spec that holds call as its only value,
// looking through parentheses around the call.
func holder(stack []ast.Node) ast.Node {
	for i := len(stack) - 2; i >= 0; i-- {
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			return stack[i]
		}
	}
	return nil
}

// commented reports whether a comment falls between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
