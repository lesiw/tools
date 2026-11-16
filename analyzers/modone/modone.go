// Package modone provides an analysis.Analyzer that reports a binary remainder
// operation with a divisor of one.
//
// The spec defines the remainder as r = x % y, where |r| < |y|
// (https://go.dev/ref/spec#Arithmetic_operators). With y equal to one, the
// only integer whose magnitude is less than one is zero, so the expression is
// zero for every integer x.
//
// The fix replaces % with * and the divisor with 0, leaving the left operand
// in place. The zero is '\x00' when the left operand has the untyped rune
// type, and 0 otherwise. An untyped int zero would change the type inferred
// for the variable in a short declaration, or the default type of an untyped
// constant. Replacing the expression with 0 would skip the left operand, so a
// call would not run and a type assertion would not panic, and would change
// the type when the operand has a defined type.
//
// The rule reports a remainder whose divisor is a constant expression with
// value one and no identifier, as in n % 1, n % 0x1, n % 1.0, n % '\x01', or
// n % (1 + 0). A divisor that contains an identifier is left alone; the rule
// reports only identifier-free constant expressions of value one. The rule
// covers only the positive divisor 1, as SA4028 does, so a
// divisor of -1 is left alone. The rule does not report 1 % n, which is not
// always zero. The rule covers binary remainders only, so a compound
// assignment such as n %= 1 is left alone. There is no fix when a comment is
// inside the divisor.
//
// The rule is adapted from staticcheck's SA4028.
package modone

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "modone",
	Doc:  "reports a binary remainder operation with a divisor of one",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || bin.Op != token.REM || !one(pass, bin.Y) {
				return true
			}
			diag := analysis.Diagnostic{
				Pos:     bin.Pos(),
				End:     bin.End(),
				Message: "the remainder is zero when the divisor is one",
			}
			if suggested, ok := fix(pass, file, bin); ok {
				diag.SuggestedFixes = []analysis.SuggestedFix{suggested}
			}
			pass.Report(diag)
			return true
		})
	}
	return nil, nil
}

// one reports whether e is a constant expression with value one and no
// identifier.
func one(pass *analysis.Pass, e ast.Expr) bool {
	v := pass.TypesInfo.Types[e].Value
	if v == nil {
		return false
	}
	v = constant.ToInt(v)
	if v.Kind() != constant.Int {
		return false
	}
	if !constant.Compare(v, token.EQL, constant.MakeInt64(1)) {
		return false
	}
	var named bool
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.Ident); ok {
			named = true
		}
		return !named
	})
	return !named
}

// zero returns a zero literal that keeps the type of bin. An untyped rune left
// operand makes the expression untyped rune, which an untyped int zero would
// not keep.
func zero(pass *analysis.Pass, bin *ast.BinaryExpr) string {
	left, ok := pass.TypesInfo.TypeOf(bin.X).(*types.Basic)
	if ok && left.Kind() == types.UntypedRune {
		return `'\x00'`
	}
	return "0"
}

// fix returns the edits that replace the operator and the divisor with a
// multiplication by zero, or no fix when the edit would delete a comment.
func fix(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) (analysis.SuggestedFix, bool) {
	if commented(file, bin.Y.Pos(), bin.Y.End()) {
		return analysis.SuggestedFix{}, false
	}
	text := zero(pass, bin)
	return analysis.SuggestedFix{
		Message: "Multiply the operand by zero",
		TextEdits: []analysis.TextEdit{
			{Pos: bin.OpPos, End: bin.OpPos + 1, NewText: []byte("*")},
			{Pos: bin.Y.Pos(), End: bin.Y.End(), NewText: []byte(text)},
		},
	}, true
}

// commented reports whether a comment overlaps the span a fix would replace.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
