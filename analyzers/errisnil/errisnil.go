// Package errisnil provides an analysis.Analyzer that reports an error
// compared with nil twice in one boolean expression.
//
// The spec's Logical operators section
// (https://go.dev/ref/spec#Logical_operators) says "The left operand is
// evaluated, and then the right if the condition requires it." The second test
// has the same result as the first, so it cannot change the result.
//
// The comparison has to test the predeclared nil and a value whose type is
// assignable to error, and the two tests have to be joined by the same
// operator. A test whose error expression contains a receive, an address of a
// composite literal, or a call other than a conversion or a pure builtin is
// left alone, and so is a repeat with such an expression between the two
// tests. The pure builtins are listed in the README. A type parameter is left
// alone unless its constraint implements error. A channel length read is left
// alone too. A repeat inside a parenthesized chain is reported; when the
// repeat is the first operand of that chain, the report has no fix. When a
// comment overlaps the text the fix would delete, the report has no fix. The
// fix keeps one test.
//
// The rule is adapted from staticcheck's SA4000 family. Every check and
// exemption is listed in the README.
package errisnil

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "errisnil",
	Doc:  "reports an error compared with nil twice in one boolean expression",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		seen := make(map[*ast.BinaryExpr]bool)
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || seen[bin] {
				return true
			}
			switch bin.Op {
			case token.LAND, token.LOR:
				check(pass, file, bin, seen)
			}
			return true
		})
	}
	return nil, nil
}

// check reports repeated tests in a chain joined by root's operator.
func check(pass *analysis.Pass, file *ast.File, root *ast.BinaryExpr, seen map[*ast.BinaryExpr]bool) {
	operands, joins := chain(root, root.Op, nil, seen)
	// reported holds the tests seen since the last effectful operand.
	var reported []comparison
	for i, operand := range operands {
		if astexpr.Effectful(pass, operand.expr) {
			reported = reported[:0]
			continue
		}
		c, ok := comparisonOf(pass, operand.expr)
		if !ok {
			continue
		}
		if repeated(pass, reported, c) {
			report(pass, file, operand, joins[i-1])
		}
		reported = append(reported, c)
	}
}

// repeated reports whether tests contains c.
func repeated(pass *analysis.Pass, tests []comparison, c comparison) bool {
	for _, test := range tests {
		if sameTest(pass, test, c) {
			return true
		}
	}
	return false
}

// sameTest reports whether a and b make the same test.
func sameTest(pass *analysis.Pass, a, b comparison) bool {
	if a.op != b.op {
		return false
	}
	if astexpr.Same(pass, a.err, b.err) {
		return true
	}
	// Text equality catches a conversion, type assertion, or pure call, which
	// Same does not compare.
	return astexpr.Text(a.err) == astexpr.Text(b.err)
}

// chainOperand is one expression joined by a chain's operator.
type chainOperand struct {
	expr  ast.Expr
	group *ast.ParenExpr
}

// chainJoin is an operator in a chain.
type chainJoin struct {
	pos   token.Pos
	group *ast.ParenExpr
}

// chain returns the operands joined by op in source order, with the operators
// between them. group is the parenthesized expression holding the chain, if
// there is one. It marks every chain node it visits.
func chain(e ast.Expr, op token.Token, group *ast.ParenExpr, seen map[*ast.BinaryExpr]bool) ([]chainOperand, []chainJoin) {
	if paren, ok := e.(*ast.ParenExpr); ok {
		bin, ok := ast.Unparen(paren).(*ast.BinaryExpr)
		if !ok || bin.Op != op {
			return []chainOperand{{expr: paren, group: group}}, nil
		}
		return chainBin(bin, op, paren, seen)
	}
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != op {
		return []chainOperand{{expr: e, group: group}}, nil
	}
	return chainBin(bin, op, group, seen)
}

// chainBin returns the operands and operators of the chain rooted at bin.
func chainBin(bin *ast.BinaryExpr, op token.Token, group *ast.ParenExpr, seen map[*ast.BinaryExpr]bool) ([]chainOperand, []chainJoin) {
	seen[bin] = true
	left, leftJoins := chain(bin.X, op, group, seen)
	return chainRight(bin.Y, op, group, seen, left, leftJoins, bin.OpPos)
}

// chainRight adds the right half of a chain and the operator before it.
func chainRight(e ast.Expr, op token.Token, group *ast.ParenExpr, seen map[*ast.BinaryExpr]bool, left []chainOperand, leftJoins []chainJoin, pos token.Pos) ([]chainOperand, []chainJoin) {
	right, rightJoins := chain(e, op, group, seen)
	return combine(left, right, leftJoins, pos, group, rightJoins)
}

// combine joins the operands and operators of the two halves of a chain.
func combine(left, right []chainOperand, leftJoins []chainJoin, pos token.Pos, group *ast.ParenExpr, rightJoins []chainJoin) ([]chainOperand, []chainJoin) {
	join := chainJoin{pos: pos, group: group}
	return append(left, right...),
		append(append(leftJoins, join), rightJoins...)
}

// comparison is a test of one error value against nil.
type comparison struct {
	op  token.Token
	err ast.Expr
}

// comparisonOf returns the error test that operand forms, and reports false
// for any other expression.
func comparisonOf(pass *analysis.Pass, operand ast.Expr) (comparison, bool) {
	bin, ok := ast.Unparen(operand).(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL && bin.Op != token.NEQ {
		return comparison{}, false
	}
	var errExpr ast.Expr
	switch {
	case isNil(pass, bin.X) == isNil(pass, bin.Y):
		return comparison{}, false
	case isNil(pass, bin.X):
		errExpr = bin.Y
	default:
		errExpr = bin.X
	}
	if !isError(pass.TypesInfo.TypeOf(errExpr)) {
		return comparison{}, false
	}
	if !astexpr.StableLength(pass, errExpr) {
		return comparison{}, false
	}
	return comparison{op: bin.Op, err: ast.Unparen(errExpr)}, true
}

// isNil reports whether e is the predeclared nil.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[ast.Unparen(e)]
	return ok && tv.IsNil()
}

// isError reports whether a value of type t is assignable to error.
func isError(t types.Type) bool {
	if t == nil {
		return false
	}
	return types.AssignableTo(t, types.Universe.Lookup("error").Type())
}

// report reports the repeated comparison at operand. join is the operator
// before it. The fix is withheld across a parenthesized group and where a
// comment overlaps the edit.
func report(pass *analysis.Pass, file *ast.File, operand chainOperand, join chainJoin) {
	d := analysis.Diagnostic{
		Pos:     operand.expr.Pos(),
		End:     operand.expr.End(),
		Message: "error compared with nil twice in the same expression",
	}
	if fixable(file, operand, join) {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Drop the repeated comparison",
			TextEdits: []analysis.TextEdit{{
				Pos: join.pos,
				End: operand.expr.End(),
			}},
		}}
	}
	pass.Report(d)
}

// fixable reports whether the fix keeps the operator and operand in one
// parenthesized group and deletes no comment.
func fixable(file *ast.File, operand chainOperand, join chainJoin) bool {
	if join.group != operand.group {
		return false
	}
	return !commented(file.Comments, join.pos, operand.expr.End())
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
