// Package mathintconv provides an analysis.Analyzer that reports a math
// rounding call on an integer converted to a float.
//
// The math package's rounding functions return an integer value. Ceil
// "returns the least integer value greater than or equal to x"
// (https://pkg.go.dev/math#Ceil). Floor returns the greatest integer value
// less than or equal to x, Round and RoundToEven return the nearest integer,
// and Trunc drops the fractional part. A conversion of an integer to float64
// produces an integer value already, so each call returns the converted
// integer unchanged.
//
// The fix replaces the call with its argument, which keeps the float64 result
// and the conversion that produces it. It is withheld where a comment sits in
// the text the edit removes, where the language requires a call, and where
// the call reaches math through a dot import, since goimports keeps an unused
// dot import. The README catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's SA4015, which reports math.Ceil,
// math.Floor, math.Trunc, math.IsNaN, and math.IsInf. IsNaN and IsInf return
// false for a converted integer, so this rule leaves them alone. It reports
// math.Round and math.RoundToEven too.
package mathintconv

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "mathintconv",
	Doc:  "reports a math rounding call on an integer converted to a float",
	Run:  run,
}

// rounding holds the math functions that leave an integer-valued argument
// unchanged.
var rounding = map[string]struct{}{
	"Ceil":        {},
	"Floor":       {},
	"Round":       {},
	"RoundToEven": {},
	"Trunc":       {},
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
				check(pass, stack, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports call when it is a rounding function applied to an integer
// converted to a float.
func check(pass *analysis.Pass, stack []ast.Node, call *ast.CallExpr) {
	file, ok := stack[0].(*ast.File)
	if !ok {
		return
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "math" {
		return
	}
	if _, ok := rounding[fn.Name()]; !ok || len(call.Args) != 1 {
		return
	}
	arg := call.Args[0]
	if !converted(pass, arg) {
		return
	}
	_, dot := ast.Unparen(call.Fun).(*ast.Ident)
	writable := !dot && !statementCall(stack) &&
		!commented(file, call.Pos(), arg.Pos()) &&
		!commented(file, arg.End(), call.End())
	var fixes []analysis.SuggestedFix
	if writable {
		fixes = []analysis.SuggestedFix{{
			Message: "Drop the call",
			TextEdits: []analysis.TextEdit{{
				Pos: call.Pos(),
				End: arg.Pos(),
			}, {
				Pos: arg.End(),
				End: call.End(),
			}},
		}}
	}
	msg := "math." + fn.Name() + " on a converted integer returns its argument"
	pass.Report(analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        msg,
		SuggestedFixes: fixes,
	})
}

// statementCall reports whether call is the expression of a statement where
// the language requires a call.
func statementCall(stack []ast.Node) bool {
	i := len(stack) - 1
	for i > 0 {
		if _, ok := stack[i-1].(*ast.ParenExpr); !ok {
			break
		}
		i--
	}
	if i == 0 {
		return false
	}
	switch stack[i-1].(type) {
	case *ast.ExprStmt, *ast.GoStmt, *ast.DeferStmt:
		return true
	}
	return false
}

// converted reports whether e converts a type whose type set holds only
// integers to a floating-point type.
func converted(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis != token.NoPos {
		return false
	}
	tv := pass.TypesInfo.Types[call.Fun]
	if !tv.IsType() || !floating(pass.TypesInfo.TypeOf(call.Fun)) {
		return false
	}
	return integer(pass.TypesInfo.TypeOf(call.Args[0]))
}

// floating reports whether t is a floating-point type.
func floating(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsFloat != 0
}

// integer reports whether every type in t's type set is an integer.
func integer(t types.Type) bool {
	if t == nil {
		return false
	}
	switch t := t.(type) {
	case *types.TypeParam:
		return integer(t.Constraint())
	case *types.Union:
		var terms int
		for term := range t.Terms() {
			terms++
			if !integer(term.Type()) {
				return false
			}
		}
		return terms > 0
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsInteger != 0
	case *types.Interface:
		var restricted bool
		for embedded := range u.EmbeddedTypes() {
			if !hasTerms(embedded) {
				continue
			}
			restricted = true
			if !integer(embedded) {
				return false
			}
		}
		return restricted
	}
	return false
}

// hasTerms reports whether t's type set is narrowed by a type term.
func hasTerms(t types.Type) bool {
	switch t := t.(type) {
	case *types.TypeParam:
		return hasTerms(t.Constraint())
	case *types.Union:
		return t.Len() > 0
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return true
	}
	for embedded := range iface.EmbeddedTypes() {
		if hasTerms(embedded) {
			return true
		}
	}
	return false
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
