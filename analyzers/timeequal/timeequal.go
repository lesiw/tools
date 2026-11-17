// Package timeequal provides an analysis.Analyzer that reports a time.Time
// compared with == or !=.
//
// The time package says what the operator does: "Note that the Go == operator
// compares not just the time instant but also the Location and the monotonic
// clock reading." The Time doc adds: "In general, prefer t.Equal(u) to t == u,
// since t.Equal uses the most accurate comparison available and correctly
// handles the case when only one of its arguments has a monotonic clock
// reading."
//
// It says that "the Time.IsZero method gives a simple way of detecting a time
// that has not been initialized explicitly", which is what a comparison
// against the zero-value literal tests for. IsZero tests the instant alone, so
// a zero instant in a named location compares unequal to time.Time{} though
// IsZero returns true.
//
// The fix writes Equal for a comparison of two time.Time values, and IsZero
// for a comparison against the zero-value literal, negated for !=. A !
// outside the comparison is folded into the replacement, so
// !(t != time.Time{}) becomes t.IsZero().
//
// The suggestion in the message, as in "; write t.IsZero()", is left out when
// the receiver or the argument prints on more than one line, when the
// comparison has a defined boolean type, or when another comparison takes the
// result of a zero-value comparison as an operand. That last shape is usually
// a deliberate test of what the operator compares, so it takes no fix either.
// The fix is withheld where a comment lies in the text it would replace, where
// the comparison spans lines, and where the rewrite would take the line past
// the width. A line already past the width still takes the fix.
//
// A case in a switch on one, a map keyed by one, a comparison of two
// zero-value literals, a comparison of a struct that holds a time, and a
// comparison of a defined type whose underlying type is time.Time, even when
// the type declares IsZero, are not reported.
//
// The rule is like revive's time-equal, which reports the comparison without a
// fix.
package timeequal

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "timeequal",
	Doc:  "reports a time.Time compared with == or !=",
	Run:  run,
}

// site is one comparison as a fix sees it.
type site struct {
	expr         ast.Expr // the node the replacement stands in for
	negate       bool     // a ! covers the comparison
	inComparison bool     // another comparison takes the result as a value
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		checkFile(pass, f)
	}
	return nil, nil
}

func checkFile(pass *analysis.Pass, f *ast.File) {
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || bin.Op != token.EQL && bin.Op != token.NEQ {
			return true
		}
		if !isTime(pass, bin.X) || !isTime(pass, bin.Y) {
			return true
		}
		report(pass, f, bin, stack)
		return true
	})
}

// report reports a comparison of two times and quotes the replacement it would
// write: Equal, or IsZero when an operand is the zero-value literal. Two
// zero-value literals leave no time under test, so they are not reported.
func report(pass *analysis.Pass, f *ast.File, bin *ast.BinaryExpr, stack []ast.Node) {
	xZero, yZero := zeroLiteral(bin.X), zeroLiteral(bin.Y)
	if xZero && yZero {
		return
	}
	s := target(stack, bin)
	from, to := s.expr.Pos(), s.expr.End()
	zero := xZero || yZero
	var message, label string
	recv := bin.X
	var arg ast.Expr
	if zero {
		message = "time.Time compared with the zero value"
		label = "Compare with IsZero"
		if xZero {
			recv = bin.Y
		}
	} else {
		message = bin.Op.String() + " on a time.Time compares more than the instant"
		label = "Compare with Equal"
		arg = bin.Y
	}
	not := bin.Op == token.NEQ
	if s.negate {
		not = !not
	}
	var text string
	isBool := astexpr.IsBool(pass.TypesInfo.TypeOf(bin))
	if isBool && !(zero && s.inComparison) {
		text = replacement(pass.Fset, recv, arg, not)
	}
	diag := analysis.Diagnostic{Pos: from, End: to, Message: message}
	if text != "" {
		diag.Message += "; write " + text
		if fits(pass, from, to, text) && !holdsComment(f, from, to) {
			diag.SuggestedFixes = []analysis.SuggestedFix{{
				Message: label,
				TextEdits: []analysis.TextEdit{{
					Pos:     from,
					End:     to,
					NewText: []byte(text),
				}},
			}}
		}
	}
	pass.Report(diag)
}

// replacement returns the text of a repair, and empty text when the call does
// not print on one line. An argument writes Equal, and no argument writes
// IsZero.
func replacement(fset *token.FileSet, recv, arg ast.Expr, not bool) string {
	name, err := receiverText(fset, recv)
	if err != nil {
		return ""
	}
	method, args := "IsZero", ""
	if arg != nil {
		call, err := render(fset, ast.Unparen(arg))
		if err != nil {
			return ""
		}
		method, args = "Equal", call
	}
	text := name + "." + method + "(" + args + ")"
	if strings.Contains(text, "\n") {
		return ""
	}
	if not {
		text = "!" + text
	}
	return text
}

// target returns the site of a fix for bin. A parenthesis or a ! is taken with
// the comparison, because the replacement binds the same way, and a parent
// comparison is recorded rather than taken.
func target(stack []ast.Node, bin *ast.BinaryExpr) site {
	s := site{expr: bin}
	for i := len(stack) - 2; i >= 0; i-- {
		switch parent := stack[i].(type) {
		case *ast.ParenExpr:
			s.expr = parent
		case *ast.UnaryExpr:
			if parent.Op != token.NOT {
				return s
			}
			s.expr, s.negate = parent, !s.negate
		case *ast.BinaryExpr:
			s.inComparison = parent.Op == token.EQL || parent.Op == token.NEQ
			return s
		default:
			return s
		}
	}
	return s
}

// receiverText returns the text of e where a selector follows it.
func receiverText(fset *token.FileSet, e ast.Expr) (string, error) {
	e = ast.Unparen(e)
	text, err := render(fset, e)
	if err != nil {
		return "", err
	}
	switch e.(type) {
	case *ast.StarExpr, *ast.UnaryExpr:
		// A selector after either binds to its operand: *p.Equal(q) is
		// *(p.Equal(q)).
		return "(" + text + ")", nil
	}
	if astexpr.Composite(e) {
		// A statement header cannot begin with a composite literal.
		return "(" + text + ")", nil
	}
	return text, nil
}

// render returns the source of one expression. It prints with the file's
// FileSet, so an expression written across lines stays across lines; a fresh
// FileSet would join them, and the analyzer would quote a replacement for an
// expression it cannot write on one line.
func render(fset *token.FileSet, e ast.Expr) (string, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, e); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// isTime reports whether e is a time.Time.
func isTime(pass *analysis.Pass, e ast.Expr) bool {
	named, ok := types.Unalias(pass.TypesInfo.TypeOf(e)).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "time" &&
		obj.Name() == "Time"
}

// zeroLiteral reports whether e is the zero time.Time literal. The caller has
// already checked that e is a time.Time, so no type is looked up here.
func zeroLiteral(e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	return ok && len(lit.Elts) == 0
}

// holdsComment reports whether a comment lies in the text a fix would replace.
func holdsComment(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if c.Pos() >= from && c.Pos() < to {
				return true
			}
		}
	}
	return false
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
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
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
