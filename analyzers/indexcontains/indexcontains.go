// Package indexcontains provides an analysis.Analyzer that reports a test of
// a strings.Index or bytes.Index result against -1.
//
// The strings package documents Index as returning "the index of the first
// instance of substr in s, or -1 if substr is not present in s", and Contains
// as reporting "whether substr is within s". bytes documents the same pair.
// strings.Index(s, sub) != -1 is true exactly when strings.Contains(s, sub) is
// true, and == -1 is true exactly when it is false.
//
// A comparison with -1 is reported on either side of the operator. With the
// call on the left, != and > test for presence, while == and <= test for
// absence. Swapping the operands swaps the operator: -1 < strings.Index(s,
// sub) tests for presence and -1 >= strings.Index(s, sub) tests for absence.
//
// The fix replaces Index with Contains and removes the comparison, adding a
// negation where the substring is absent. It is withheld when a comment sits
// in the comparison text the rewrite removes.
//
// A comparison with another value is left alone, as is a comparison that is
// always true or always false. The value must be the literal -1: a named
// constant or a constant expression is left alone, and rewriting a name would
// remove its use. Other functions, such as IndexAny, IndexRune, and IndexByte,
// are not reported. A call through a dot import, a function value, or a value
// that shadows either package name is left alone as well.
//
// The rule is adapted from staticcheck's S1003, which also covers IndexAny and
// IndexRune and the comparisons with zero.
package indexcontains

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "indexcontains",
	Doc:  "reports a test of an Index result against -1",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			check(pass, file, bin)
			return true
		})
	}
	return nil, nil
}

// check reports one comparison of an Index result with -1.
func check(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) {
	var (
		sel      *ast.SelectorExpr
		callLeft bool
	)
	if s, ok := indexCall(pass, bin.X); ok && isNegativeOne(bin.Y) {
		sel, callLeft = s, true
	} else if s, ok := indexCall(pass, bin.Y); ok && isNegativeOne(bin.X) {
		sel, callLeft = s, false
	} else {
		return
	}
	contains, ok := meaning(bin.Op, callLeft)
	if !ok {
		return
	}
	message := "Index tested against -1; use !Contains"
	if contains {
		message = "Index tested against -1; use Contains"
	}
	d := analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: message,
	}
	if fix := replacement(file, bin, sel, callLeft, contains); fix != nil {
		d.SuggestedFixes = []analysis.SuggestedFix{*fix}
	}
	pass.Report(d)
}

// indexCall returns the selector of a strings.Index or bytes.Index call.
func indexCall(pass *analysis.Pass, e ast.Expr) (*ast.SelectorExpr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return nil, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Index" {
		return nil, false
	}
	ident, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return nil, false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok {
		return nil, false
	}
	switch pkg.Imported().Path() {
	case "strings", "bytes":
		return sel, true
	}
	return nil, false
}

// isNegativeOne reports whether e is the integer literal -1, with or without
// parentheses.
func isNegativeOne(e ast.Expr) bool {
	unary, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB {
		return false
	}
	lit, ok := ast.Unparen(unary.X).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	n, err := strconv.ParseInt(lit.Value, 0, 64)
	return err == nil && n == 1
}

// meaning returns whether the comparison tests for presence (contains is
// true) or absence (contains is false), and whether it is a test at all.
func meaning(op token.Token, callLeft bool) (contains, ok bool) {
	if !callLeft {
		op = mirror(op)
	}
	switch op {
	case token.NEQ, token.GTR:
		return true, true
	case token.EQL, token.LEQ:
		return false, true
	}
	return false, false
}

// mirror returns op with its operands exchanged.
func mirror(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.GTR:
		return token.LSS
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	}
	return op
}

// replacement returns the fix that swaps Index for Contains, or nil when a
// comment sits in the text the fix removes.
func replacement(file *ast.File, bin *ast.BinaryExpr, sel *ast.SelectorExpr, callLeft, contains bool) *analysis.SuggestedFix {
	side := bin.X
	if !callLeft {
		side = bin.Y
	}
	lo, hi := side.End(), bin.End()
	if !callLeft {
		lo, hi = bin.Pos(), side.Pos()
	}
	if commented(file, lo, hi) {
		return nil
	}
	edits := []analysis.TextEdit{{
		Pos:     sel.Sel.Pos(),
		End:     sel.Sel.End(),
		NewText: []byte("Contains"),
	}}
	if callLeft {
		edits = append(edits, analysis.TextEdit{
			Pos: side.End(),
			End: bin.End(),
		})
	} else {
		edits = append(edits, analysis.TextEdit{
			Pos: bin.Pos(),
			End: side.Pos(),
		})
	}
	if !contains {
		edits = append(edits, analysis.TextEdit{
			Pos:     side.Pos(),
			End:     side.Pos(),
			NewText: []byte("!"),
		})
	}
	return &analysis.SuggestedFix{
		Message:   "Compare with Contains",
		TextEdits: edits,
	}
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
