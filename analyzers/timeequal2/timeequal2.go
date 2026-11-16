// Package timeequal2 provides an analysis.Analyzer that reports a comparison
// of a time.Time with the zero-value literal.
//
// The Time doc says: "The zero value of type Time is January 1, year 1,
// 00:00:00.000000000 UTC. As this time is unlikely to come up in practice,
// the Time.IsZero method gives a simple way of detecting a time that has not
// been initialized explicitly." IsZero tests the instant alone. A comparison
// with == also compares the location and the monotonic clock reading, so a
// zero instant in a named location can compare unequal to the literal while
// IsZero returns true. A comparison with the literal is usually a test for an
// unset time, and IsZero is that test.
//
// A comparison used as an operand of another comparison is reported without a
// fix, because the outer comparison uses the result as a value.
//
// The fix writes IsZero, negated for !=. It is withheld where the comparison
// has a defined boolean type, is an operand of another comparison, or the edit
// would drop a comment, join lines, or take the line past the width. A !
// outside the comparison is folded into the replacement, so
// !(t != time.Time{}) becomes t.IsZero().
//
// The rule is like revive's time-equal, which reports a time.Time compared
// with == or != without a fix.
package timeequal2

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "timeequal2",
	Doc:  "reports a time.Time compared with the zero-value literal",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		checkFile(pass, file)
	}
	return nil, nil
}

func checkFile(pass *analysis.Pass, file *ast.File) {
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || bin.Op != token.EQL && bin.Op != token.NEQ {
			return true
		}
		if !isTimeValue(pass, bin.X) || !isTimeValue(pass, bin.Y) {
			return true
		}
		xZero, yZero := zeroLiteral(bin.X), zeroLiteral(bin.Y)
		if xZero == yZero {
			// Neither operand is the literal, or both are. Neither case has
			// a time under test.
			return true
		}
		span, negate := target(stack, bin)
		report(pass, file, bin, span, xZero, comparisonOperand(stack), negate)
		return true
	})
}

// comparisonOperand reports whether another comparison takes the expression
// at the top of stack as an operand, as in (t == time.Time{}) == (u ==
// time.Time{}). The outer comparison uses the inner result as a value, so a
// fix would change the value that comparison receives.
func comparisonOperand(stack []ast.Node) bool {
	for i := len(stack) - 2; i >= 0; i-- {
		switch parent := stack[i].(type) {
		case *ast.ParenExpr:
		case *ast.UnaryExpr:
			if parent.Op != token.NOT {
				return false
			}
		case *ast.BinaryExpr:
			return parent.Op == token.EQL || parent.Op == token.NEQ
		default:
			return false
		}
	}
	return false
}

// target returns the node a fix replaces and whether a ! covers it. A
// parenthesis or a ! is taken with the comparison because the replacement
// binds the same way.
func target(stack []ast.Node, bin *ast.BinaryExpr) (ast.Expr, bool) {
	var (
		span   ast.Expr = bin
		negate bool
	)
	for i := len(stack) - 2; i >= 0; i-- {
		switch parent := stack[i].(type) {
		case *ast.ParenExpr:
			span = parent
		case *ast.UnaryExpr:
			if parent.Op != token.NOT {
				return span, negate
			}
			span, negate = parent, !negate
		default:
			return span, negate
		}
	}
	return span, negate
}

func report(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, span ast.Expr, xZero, nested, negate bool) {
	var (
		from, to = span.Pos(), span.End()
		recv     = bin.X
		not      = bin.Op == token.NEQ
		diag     = analysis.Diagnostic{
			Pos:     from,
			End:     to,
			Message: "time.Time compared with the zero value",
		}
	)
	if xZero {
		recv = bin.Y
	}
	if negate {
		not = !not
	}
	if !astexpr.IsBool(pass.TypesInfo.TypeOf(bin)) {
		pass.Report(diag)
		return
	}
	if nested {
		pass.Report(diag)
		return
	}
	text, ok := receiverText(recv)
	if !ok {
		// A diagnostic message is one line, so a receiver on more than one
		// line is not quoted.
		pass.Report(diag)
		return
	}
	if not {
		text = "!" + text
	}
	text += ".IsZero()"
	diag.Message += "; write " + text
	if fixable(file, from, to) && fits(pass, from, to, text) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Compare with IsZero",
			TextEdits: []analysis.TextEdit{{
				Pos:     from,
				End:     to,
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
}

// receiverText returns the text of e where a selector follows it.
func receiverText(e ast.Expr) (string, bool) {
	e = ast.Unparen(e)
	text, ok := astexpr.OneLine(e)
	if !ok {
		return "", false
	}
	switch e.(type) {
	case *ast.StarExpr, *ast.UnaryExpr:
		// A selector after either binds to its operand: *p.IsZero is
		// *(p.IsZero).
		return "(" + text + ")", true
	}
	if astexpr.Composite(e) {
		// A statement header cannot begin with a composite literal.
		return "(" + text + ")", true
	}
	return text, true
}

// fixable reports whether the range from to contains no comment, so that the
// rewrite does not drop one.
func fixable(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < to && from < group.End() {
			return false
		}
	}
	return true
}

// fits reports whether text may replace the range from to. The range lies on
// one line, and the rewritten line is within the width unless the line is
// already past it.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	// Written positions, which a //line directive does not move.
	start, end := pass.Fset.PositionFor(from, false),
		pass.Fset.PositionFor(to, false)
	if start.Line != end.Line {
		return false
	}
	if linelen.Config.Len <= 0 {
		return true
	}
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	// A driver that parses other bytes than ReadFile returns can put the
	// position outside the line, so the bounds are checked before use.
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
	line := lines[start.Line-1]
	if start.Column > len(line)+1 || end.Column > len(line)+1 {
		return false
	}
	if !linelen.CheckLine(line) {
		return true
	}
	var rewritten bytes.Buffer
	rewritten.Write(line[:start.Column-1])
	rewritten.WriteString(text)
	rewritten.Write(line[end.Column-1:])
	return linelen.CheckLine(rewritten.Bytes())
}

// isTimeValue reports whether e is a time.Time.
func isTimeValue(pass *analysis.Pass, e ast.Expr) bool {
	named, ok := types.Unalias(pass.TypesInfo.TypeOf(e)).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "time" &&
		obj.Name() == "Time"
}

// zeroLiteral reports whether e is an empty time.Time composite literal.
func zeroLiteral(e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	return ok && len(lit.Elts) == 0
}
