// Package lowerfold provides an analysis.Analyzer that reports a comparison
// of two strings converted to lower or upper case.
//
// strings.ToLower "returns s with all Unicode letters mapped to their lower
// case" and strings.ToUpper does the same for upper case. Each call returns a
// new string when the text changes, so a comparison of two such results builds
// a string for each side. strings.EqualFold "reports whether s and t,
// interpreted as UTF-8 strings, are equal under simple Unicode case-folding"
// and takes the two originals.
//
// A comparison is reported when both sides call the same function, ToLower or
// ToUpper in strings, and the operator is == or !=. A conversion on one side
// only, a pair that uses different functions, an ordering comparison, a call
// reached through a dot import, and a function value are left alone.
//
// Folding and conversion differ for some code points. strings.ToLower("Σ") is
// "σ" and strings.ToLower("ς") is "ς", so the comparison is false, while
// strings.EqualFold compares the two as equal. Go issue 52204 describes the
// difference.
//
// The fix writes EqualFold, qualified with the file's name for the strings
// import, and negates it for !=. It is withheld where a comment lies in the
// text the rewrite would drop, and where the comparison spans lines.
//
// The rule is adapted from staticcheck's SA6005.
package lowerfold

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "lowerfold",
	Doc:  "reports a comparison of two strings converted to lower or upper case",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || bin.Op != token.EQL && bin.Op != token.NEQ {
				return true
			}
			check(pass, file, src, bin)
			return true
		})
	}
	return nil, nil
}

// conversion is a call to one of the two strings functions.
type conversion struct {
	qual string
	fn   string
	call *ast.CallExpr
}

// check reports one comparison when both sides convert the same way.
func check(pass *analysis.Pass, file *ast.File, src []byte, bin *ast.BinaryExpr) {
	left, ok := conversionOf(pass, bin.X)
	if !ok {
		return
	}
	right, ok := conversionOf(pass, bin.Y)
	if !ok || left.fn != right.fn {
		return
	}
	not := bin.Op == token.NEQ
	msg := left.qual + "." + left.fn + " on both sides; use "
	if not {
		msg += "!"
	}
	msg += left.qual + ".EqualFold"
	diag := analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: msg,
	}
	if fix, ok := replacement(pass, file, src, bin, left, right, not); ok {
		diag.SuggestedFixes = []analysis.SuggestedFix{fix}
	}
	pass.Report(diag)
}

// conversionOf returns the conversion a call performs, and false for any other
// expression.
func conversionOf(pass *analysis.Pass, e ast.Expr) (conversion, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return conversion{}, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return conversion{}, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return conversion{}, false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "strings" {
		return conversion{}, false
	}
	if sel.Sel.Name != "ToLower" && sel.Sel.Name != "ToUpper" {
		return conversion{}, false
	}
	return conversion{qual: pkg.Name(), fn: sel.Sel.Name, call: call}, true
}

// replacement returns the fix that writes EqualFold, and reports false where
// the source is not available, a comment would go with the removed text, or
// the comparison spans lines.
func replacement(pass *analysis.Pass, file *ast.File, src []byte, bin *ast.BinaryExpr, left, right conversion, not bool) (analysis.SuggestedFix, bool) {
	start, end := pass.Fset.Position(bin.Pos()), pass.Fset.Position(bin.End())
	if start.Line != end.Line {
		return analysis.SuggestedFix{}, false
	}
	first, ok := text(pass, src, left.call.Args[0])
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	second, ok := text(pass, src, right.call.Args[0])
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	if commented(file, bin, left.call.Args[0], right.call.Args[0]) {
		return analysis.SuggestedFix{}, false
	}
	call := left.qual + ".EqualFold(" + first + ", " + second + ")"
	message := "Replace with " + left.qual + ".EqualFold"
	if not {
		call = "!" + call
		message = "Replace with !" + left.qual + ".EqualFold"
	}
	return analysis.SuggestedFix{
		Message: message,
		TextEdits: []analysis.TextEdit{{
			Pos:     bin.Pos(),
			End:     bin.End(),
			NewText: []byte(call),
		}},
	}, true
}

// text returns the source of expr.
func text(pass *analysis.Pass, src []byte, expr ast.Expr) (string, bool) {
	tokFile := pass.Fset.File(expr.Pos())
	if tokFile == nil {
		return "", false
	}
	start, end := tokFile.Offset(expr.Pos()), tokFile.Offset(expr.End())
	if start < 0 || end < start || end > len(src) {
		return "", false
	}
	return string(src[start:end]), true
}

// commented reports whether a comment in bin lies outside its two arguments,
// which are the only parts of it the fix keeps.
func commented(file *ast.File, bin *ast.BinaryExpr, args ...ast.Expr) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() < bin.Pos() || comment.End() > bin.End() {
				continue
			}
			var kept bool
			for _, arg := range args {
				if comment.Pos() >= arg.Pos() && comment.End() <= arg.End() {
					kept = true
					break
				}
			}
			if !kept {
				return true
			}
		}
	}
	return false
}
