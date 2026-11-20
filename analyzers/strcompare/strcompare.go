// Package strcompare provides an analysis.Analyzer that reports a comparison
// of a strings.Compare or bytes.Compare result with a constant.
//
// strings.Compare documents both halves of the rule: "The result will be 0 if
// a == b, -1 if a < b, and +1 if a > b", and "it is usually clearer and always
// faster to use the built-in string comparison operators ==, <, >, and so on."
// bytes.Compare has no such note, and a byte slice has no comparison operator,
// so only its zero test has a shorter form: bytes.Equal.
//
// The fix writes the direct comparison, or the bytes.Equal call for a slice,
// and is withheld where the edit would delete a comment or an argument has no
// one-line form. The README catalogs every check, exemption, and fix.
//
// The rule is inspired by go-critic's stringsCompare. The bytes.Compare arm is
// adapted from staticcheck's S1004, which reads only the comparison with the
// call and the literal zero in that order.
package strcompare

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "strcompare",
	Doc:  "reports a comparison of a Compare result with a constant",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if bin, ok := n.(*ast.BinaryExpr); ok {
				report(pass, file, bin)
			}
			return true
		})
	}
	return nil, nil
}

// report checks one comparison and reports it when it calls Compare.
func report(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) {
	var (
		args, qual, isBytes, ok = compareCall(pass, bin.X)
		other                   = bin.Y
		written                 = bin.Op
	)
	if !ok {
		args, qual, isBytes, ok = compareCall(pass, bin.Y)
		other, written = bin.X, mirror(bin.Op)
	}
	if !ok {
		return
	}
	c, isLiteral := intValue(other)
	if !isLiteral {
		return
	}
	// written is the comparison as the code has it; op is the one the fix
	// writes, which a slice takes from the written one.
	op := written
	if isBytes {
		if written != token.EQL && written != token.NEQ || c != 0 {
			return
		}
	} else {
		var found bool
		if op, found = comparison(written, c); !found {
			return
		}
	}
	// negate is the ! the fix and the advice both carry, or nothing.
	var negate string
	if isBytes && written == token.NEQ {
		negate = "!"
	}
	message, label := "comparison through Compare; compare directly", "Compare directly"
	if isBytes {
		label = "Call " + negate + qual + ".Equal"
		message = "Compare tested against zero; use " + negate + "bytes.Equal"
	}
	diag := analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: message,
	}
	if text, ok := rewrite(args, qual, op, isBytes); ok {
		if !commented(file, bin.Pos(), bin.End()) {
			diag.SuggestedFixes = []analysis.SuggestedFix{{
				Message: label,
				TextEdits: []analysis.TextEdit{{
					Pos:     bin.Pos(),
					End:     bin.End(),
					NewText: []byte(negate + text),
				}},
			}}
		}
	}
	pass.Report(diag)
}

// compareCall returns the arguments and the package qualifier of a
// strings.Compare or bytes.Compare call, and whether the function is
// bytes.Compare, whose only shorter form is a zero test.
func compareCall(pass *analysis.Pass, e ast.Expr) (args []ast.Expr, qual string, isBytes, ok bool) {
	call, isCall := ast.Unparen(e).(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return nil, "", false, false
	}
	sel, isSel := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !isSel || sel.Sel.Name != "Compare" {
		return nil, "", false, false
	}
	ident, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return nil, "", false, false
	}
	pkg, isPkg := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !isPkg {
		return nil, "", false, false
	}
	switch pkg.Imported().Path() {
	case "strings":
		return call.Args, astexpr.Text(sel.X), false, true
	case "bytes":
		return call.Args, astexpr.Text(sel.X), true, true
	}
	return nil, "", false, false
}

// comparison returns the operator that says what a Compare result tested with
// op against c says about the two strings, and whether one exists. Compare
// returns -1, 0, or 1, so a constant outside that range answers the test
// wherever it is evaluated.
func comparison(op token.Token, c int64) (token.Token, bool) {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
	default:
		return token.ILLEGAL, false
	}
	switch c {
	case 0:
		return op, true
	case -1:
		switch op {
		case token.EQL, token.LEQ:
			return token.LSS, true
		case token.NEQ, token.GTR:
			return token.GEQ, true
		}
	case 1:
		switch op {
		case token.EQL, token.GEQ:
			return token.GTR, true
		case token.NEQ, token.LSS:
			return token.LEQ, true
		}
	}
	return token.ILLEGAL, false
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

// rewrite returns the text that replaces a comparison: the comparison op for
// strings.Compare, and a call to Equal for bytes.Compare. It reports false
// when an argument does not print on one line.
func rewrite(args []ast.Expr, qual string, op token.Token, isBytes bool) (string, bool) {
	var text [2]string
	for i, arg := range args {
		one, ok := astexpr.OneLine(arg)
		if !ok {
			return "", false
		}
		text[i] = one
	}
	if !isBytes {
		return text[0] + " " + op.String() + " " + text[1], true
	}
	return qual + ".Equal(" + text[0] + ", " + text[1] + ")", true
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

// intValue returns the value of e when e is an integer literal, with or
// without a sign.
func intValue(e ast.Expr) (int64, bool) {
	var negate bool
	if unary, ok := ast.Unparen(e).(*ast.UnaryExpr); ok {
		if unary.Op != token.SUB && unary.Op != token.ADD {
			return 0, false
		}
		negate, e = unary.Op == token.SUB, unary.X
	}
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	n, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil {
		return 0, false
	}
	if negate {
		return -n, true
	}
	return n, true
}
