// Package timeuntil provides an analysis.Analyzer that reports a comparison
// of time.Now().Add(d) with a time.Time. The same test is a comparison of
// time.Until(t) or time.Since(t) with a duration.
//
// The time package documentation gives the shorthand for each direction:
// "Until returns the duration until t. It is shorthand for t.Sub(time.Now())"
// and "Since returns the time elapsed since t. It is shorthand for
// time.Now().Sub(t)".
//
// time.Now().Add(d).Before(t) and t.After(time.Now().Add(d)) are
// time.Until(t) > d; time.Now().Add(d).After(t) and
// t.Before(time.Now().Add(d)) are time.Until(t) < d.
//
// An Add argument of the form -d is written with Since when d and -d both fit
// in a time.Duration; otherwise it is written with Until, with the minus left
// in place. When time.Now().Add(d) is the receiver, a comparison is left alone
// if the time operand or the duration has an effect: a call other than a
// conversion or a free builtin, a channel receive, or the address of a
// composite literal. The rewrite evaluates the time operand before the clock
// read and before the duration. With the call as the argument, the calls keep
// their order, but a receiver value may be read after the arguments; an
// addressable or pointer time operand, or a conversion or type assertion, is
// left alone there when the duration has a side effect. The fix is withheld
// where the comparison spans lines or contains a comment, and where the
// replacement prints on more than one line; the last case is reported without
// its operands.
//
// The rewrite compares durations, where the original compares instants.
// time.Until and time.Since saturate at the int64 limits, so a span beyond
// about 292 years can have a different answer.
//
// The rule is adapted from staticcheck's S1024, which reports
// t.Sub(time.Now()) alone. This rule reports the same test written with Before
// and After; nowsub reports t.Sub(time.Now()) and time.Now().Sub(t).
package timeuntil

import (
	"go/ast"
	"go/constant"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/timeexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "timeuntil",
	Doc:  "reports time.Now().Add(d) compared with a time.Time",
	Run:  run,
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
				check(pass, file, call, stack)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a Before or After call that compares time.Now().Add(d) with a
// time.Time.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stack []ast.Node) {
	sel, method, ok := comparison(pass, call)
	if !ok {
		return
	}
	shift, other, left, ok := sides(pass, call, sel)
	if !ok {
		return
	}
	if left && effects(pass, other, shift.duration) {
		return
	}
	if reordered(pass, left, other, shift.duration) {
		return
	}
	otherText, ok := timeexpr.Text(pass, other)
	if !ok {
		return
	}
	span, negate, parent := target(stack, call)
	if discarded(parent, span) {
		return
	}
	report(pass, file, finding{
		span: span, parent: parent, negate: negate,
		method: method, left: left, shift: shift, otherText: otherText,
	})
}

// comparison returns the Before or After call's selector and method name.
func comparison(pass *analysis.Pass, call *ast.CallExpr) (*ast.SelectorExpr, string, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 {
		return nil, "", false
	}
	method := sel.Sel.Name
	if method != "Before" && method != "After" {
		return nil, "", false
	}
	return sel, method, timeexpr.IsFunc(pass, sel.Sel, method)
}

// reordered reports whether the rewrite reads the time operand at a different
// point when the duration has a side effect. A receiver may be read after the
// arguments, and a pointer is read through at the call.
func reordered(pass *analysis.Pass, left bool, other, duration ast.Expr) bool {
	if left || !astexpr.Effectful(pass, duration) {
		return false
	}
	if timeexpr.IsPointer(pass.TypesInfo.TypeOf(other)) {
		return true
	}
	switch e := ast.Unparen(other).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr,
		*ast.TypeAssertExpr:
		return true
	case *ast.CallExpr:
		tv, ok := pass.TypesInfo.Types[e.Fun]
		return ok && tv.IsType()
	}
	return false
}

// effects reports whether evaluating the time operand or the duration can have
// a side effect.
func effects(pass *analysis.Pass, other, duration ast.Expr) bool {
	return astexpr.Effectful(pass, other) || astexpr.Effectful(pass, duration)
}

// finding is one comparison and the parts of its rewrite.
type finding struct {
	span      ast.Expr
	parent    ast.Node
	negate    bool
	method    string
	left      bool
	shift     *shift
	otherText string
}

// report emits the diagnostic and, where the edit is safe, its fix.
func report(pass *analysis.Pass, file *ast.File, f finding) {
	before, op := f.method == "Before", "<"
	if before == f.left {
		op = ">"
	}
	arg := f.shift.duration
	helper := f.shift.qual + ".Until"
	if e, ok := negated(arg); ok && canNegate(pass, e) {
		arg = e
		helper = f.shift.qual + ".Since"
		op = reverse(op)
	}
	if f.negate {
		op = flip(op)
	}
	argText, oneLine := astexpr.OneLine(arg)
	if !oneLine || strings.ContainsRune(f.otherText, '\n') {
		pass.Report(analysis.Diagnostic{
			Pos:     f.span.Pos(),
			End:     f.span.End(),
			Message: "comparison can be replaced by a comparison with " + helper,
		})
		return
	}
	text := helper + "(" + f.otherText + ") " + op + " " + argText
	if needsParens(f.parent) {
		text = "(" + text + ")"
	}
	diag := analysis.Diagnostic{
		Pos:     f.span.Pos(),
		End:     f.span.End(),
		Message: "comparison can be replaced by " + text,
	}
	if timeexpr.Fixable(pass.Fset, file, f.span) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace with " + text,
			TextEdits: []analysis.TextEdit{{
				Pos:     f.span.Pos(),
				End:     f.span.End(),
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
}

// shift is the duration of one time.Now().Add(d) call and the name the file
// uses for the time package.
type shift struct {
	duration ast.Expr
	qual     string
}

// sides returns the shift, the time it is compared with, whether the shift is
// the receiver, and whether the call has either shape.
func sides(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr) (_ *shift, other ast.Expr, left, ok bool) {
	if shift, ok := nowAdd(pass, sel.X); ok {
		return shift, call.Args[0], true, true
	}
	if shift, ok := nowAdd(pass, call.Args[0]); ok {
		return shift, sel.X, false, true
	}
	return nil, nil, false, false
}

// nowAdd returns the duration of a time.Now().Add(d) call.
func nowAdd(pass *analysis.Pass, e ast.Expr) (*shift, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || !timeexpr.IsFunc(pass, sel.Sel, "Add") {
		return nil, false
	}
	qual, ok := timeexpr.NowQualifier(pass, sel.X)
	if !ok {
		return nil, false
	}
	return &shift{duration: call.Args[0], qual: qual}, true
}

// target returns the node a fix replaces, whether a ! covers it, and the
// parent of that node. A parenthesis is included so the fix replaces the whole
// parenthesized expression, and a ! is included so the comparison can become
// the equivalent non-strict form.
func target(stack []ast.Node, call *ast.CallExpr) (ast.Expr, bool, ast.Node) {
	var (
		span   ast.Expr = call
		negate bool
	)
	for i := len(stack) - 2; i >= 0; i-- {
		switch parent := stack[i].(type) {
		case *ast.ParenExpr:
			span = parent
		case *ast.UnaryExpr:
			if parent.Op != token.NOT {
				return span, negate, parent
			}
			span, negate = parent, !negate
		default:
			return span, negate, parent
		}
	}
	return span, negate, nil
}

// discarded reports whether the target is a call used as an expression
// statement, or as the call of a go or defer statement, where its result is
// discarded.
func discarded(parent ast.Node, span ast.Expr) bool {
	switch parent := parent.(type) {
	case *ast.ExprStmt:
		return parent.X == span
	case *ast.GoStmt:
		return parent.Call == span
	case *ast.DeferStmt:
		return parent.Call == span
	}
	return false
}

// negated returns the operand of a duration written with a leading minus.
func negated(e ast.Expr) (ast.Expr, bool) {
	e = ast.Unparen(e)
	unary, ok := e.(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB {
		return nil, false
	}
	return ast.Unparen(unary.X), true
}

// needsParens reports whether parentheses are required for the replacement to
// keep its grouping as an operand of == or !=.
func needsParens(parent ast.Node) bool {
	bin, ok := parent.(*ast.BinaryExpr)
	return ok && (bin.Op == token.EQL || bin.Op == token.NEQ)
}

// canNegate reports whether e is a constant whose integer value fits in int64.
// A variable's value is not known.
func canNegate(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil {
		return false
	}
	v := constant.ToInt(tv.Value)
	if v.Kind() != constant.Int {
		return false
	}
	_, ok = constant.Int64Val(v)
	return ok
}

// reverse returns the operator for the comparison in the other direction.
func reverse(op string) string {
	if op == ">" {
		return "<"
	}
	return ">"
}

// flip returns the operator for the negated comparison with op.
func flip(op string) string {
	if op == ">" {
		return "<="
	}
	return ">="
}
