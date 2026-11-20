// Package emptystring provides an analysis.Analyzer that reports a test of
// whether a string is empty, written as a comparison of its length with zero.
//
// The String types section of the spec says the length of a string is never
// negative (https://go.dev/ref/spec#String_types). So len(s) == 0 and
// len(s) <= 0 both become s == "", and len(s) != 0 and len(s) > 0 both become
// s != ""; a comparison with "" is the form a reader expects. The rule
// matches either operand order, so 0 == len(s) is reported too.
//
// The zero must be the literal 0, and len must be the builtin. The operand's
// type must be a string, or a type whose underlying type is a string; a
// comparison whose operand is a type parameter is not reported.
//
// The fix is withheld when the operand's own text spans more than one line,
// since the printed form would put it on one; when the printed form has a
// newline, as an anonymous struct type has; and when a comment is inside the
// comparison, since the rewrite would drop it. A comparison that wraps while
// its operand does not still takes the fix.
//
// An operand that holds a composite literal is parenthesized, since the
// comparison may sit in a statement header.
//
// The rule is inspired by go-critic's emptyStringTest, which matches the same
// four comparisons with the length on the left.
//
// len(s) < 0 is always false and len(s) >= 0 is always true; builtinbound
// reports these comparisons in either order and replaces them with the
// constant.
package emptystring

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptystring",
	Doc:  "reports a test of whether a string is empty, written as a comparison of its length with zero",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			if str, op, ok := test(pass, bin); ok {
				report(pass, file, bin, str, op)
			}
			return true
		})
	}
	return nil, nil
}

// test returns the string operand and the operator to write when bin tests a
// string's emptiness. len is never negative, so < is always false and >= is
// always true; builtinbound reports those two.
func test(pass *analysis.Pass, bin *ast.BinaryExpr) (ast.Expr, token.Token, bool) {
	var (
		str, ok = stringOperand(pass, bin.X, bin.Y)
		op      = bin.Op
	)
	if !ok {
		str, ok = stringOperand(pass, bin.Y, bin.X)
		op = astexpr.Flip(op)
	}
	if !ok {
		return nil, 0, false
	}
	switch op {
	case token.EQL, token.LEQ:
		return str, token.EQL, true
	case token.NEQ, token.GTR:
		return str, token.NEQ, true
	}
	return nil, 0, false
}

// stringOperand returns the operand of a call to the builtin len over a string
// when other is the literal 0, ignoring parentheses on either side.
func stringOperand(pass *analysis.Pass, call, other ast.Expr) (ast.Expr, bool) {
	call, other = ast.Unparen(call), ast.Unparen(other)
	lit, ok := other.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT || lit.Value != "0" {
		return nil, false
	}
	expr, ok := call.(*ast.CallExpr)
	if !ok || len(expr.Args) != 1 {
		return nil, false
	}
	id, ok := expr.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("len") {
		return nil, false
	}
	typ := pass.TypesInfo.TypeOf(expr.Args[0])
	if typ == nil {
		return nil, false
	}
	basic, ok := typ.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsString == 0 {
		return nil, false
	}
	return expr.Args[0], true
}

// report reports bin, with the rewrite as its suggested fix when one is
// offered.
func report(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, str ast.Expr, op token.Token) {
	var (
		from, to = bin.Pos(), bin.End()
		diag     = analysis.Diagnostic{
			Pos:     from,
			End:     to,
			Message: `compare the string with "" instead`,
		}
	)
	if text, ok := rewrite(pass, file, bin, str, op); ok {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Compare the string",
			TextEdits: []analysis.TextEdit{{
				Pos:     from,
				End:     to,
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
}

// rewrite returns the text that replaces bin, and reports false where the fix
// would join lines, drop a comment, or reflow the operand.
func rewrite(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, str ast.Expr, op token.Token) (string, bool) {
	// Written positions, which a //line directive does not move.
	var (
		start, end = pass.Fset.PositionFor(str.Pos(), false),
			pass.Fset.PositionFor(str.End(), false)
		commented = astexpr.Commented(file, bin.Pos(), bin.End())
	)
	if start.Line != end.Line || commented {
		return "", false
	}
	text, ok := astexpr.OneLine(str)
	if !ok {
		return "", false
	}
	if composite(str) {
		text = "(" + text + ")"
	}
	return text + " " + op.String() + ` ""`, true
}

// composite reports whether e holds a composite literal where a statement
// header cannot have one. A literal inside a call argument, a pair of
// parentheses, or an index is allowed.
func composite(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		return composite(e.X) || composite(e.Y)
	case *ast.SelectorExpr:
		return composite(e.X)
	case *ast.IndexExpr:
		return composite(e.X)
	case *ast.IndexListExpr:
		return composite(e.X)
	case *ast.SliceExpr:
		return composite(e.X)
	case *ast.TypeAssertExpr:
		return composite(e.X)
	case *ast.CallExpr:
		return composite(e.Fun)
	case *ast.CompositeLit:
		return true
	}
	return false
}
