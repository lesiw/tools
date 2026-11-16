// Package nancompare provides an analysis.Analyzer that reports a comparison
// with math.NaN().
//
// The math package defines both functions: "NaN returns an IEEE 754
// 'not-a-number' value" and "IsNaN reports whether f is an IEEE 754
// 'not-a-number' value." The spec compares floating-point values "as defined
// by the IEEE 754 standard". Under that standard a NaN is unequal to every
// value, including itself, and every ordering comparison with a NaN is false.
// So ==, <, <=, >, and >= are false when either operand is NaN, and != is
// true.
//
// A comparison is reported when one side is a call to math.NaN() or a
// conversion of one to a floating-point type. A case in an expression switch
// that compares its tag with NaN is reported, and so is a switch whose tag is
// NaN; no case can match.
//
// A NaN held in a variable is left alone; values are not tracked across
// statements.
//
// The fix writes math.IsNaN(x), or !math.IsNaN(x) for !=, when the other
// operand is assignable to float64 or is a float type. It is withheld for an
// operand that is neither, for an ordering comparison, when both sides are
// NaN, for a file that dot-imports math, where the comparison spans lines or
// holds a comment, where the operand does not print on one line, where the
// rewrite would take the line past the width, and where a name the rewrite
// writes reaches another object. A switch case is reported without a fix.
//
// The rule is adapted from staticcheck's SA4012. The README catalogs every
// check and exemption.
package nancompare

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "nancompare",
	Doc:  "reports a comparison with math.NaN()",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BinaryExpr:
				if isComparison(n.Op) {
					check(pass, file, n)
				}
			case *ast.SwitchStmt:
				checkSwitch(pass, n)
			}
			return true
		})
	}
	return nil, nil
}

// isComparison reports whether op is a comparison operator.
func isComparison(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	}
	return false
}

// check reports bin when one side is math.NaN().
func check(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) {
	var (
		left, leftNaN   = nanValue(pass, bin.X)
		right, rightNaN = nanValue(pass, bin.Y)
	)
	switch {
	case leftNaN && rightNaN:
		// There is no operand for the fix to test.
		pass.Report(diagnostic(pass, file, bin, nil, nil))
	case leftNaN:
		pass.Report(diagnostic(pass, file, bin, bin.Y, left))
	case rightNaN:
		pass.Report(diagnostic(pass, file, bin, bin.X, right))
	}
}

// checkSwitch reports a switch whose tag or case compares with NaN.
func checkSwitch(pass *analysis.Pass, stmt *ast.SwitchStmt) {
	if stmt.Tag != nil {
		if _, ok := nanValue(pass, stmt.Tag); ok {
			pass.Report(analysis.Diagnostic{
				Pos:     stmt.Tag.Pos(),
				End:     stmt.Tag.End(),
				Message: "no value equals NaN; no case can match",
			})
			return
		}
	}
	checkCases(pass, stmt)
}

// checkCases reports a case in a switch that compares the tag with NaN.
func checkCases(pass *analysis.Pass, stmt *ast.SwitchStmt) {
	if stmt.Tag == nil {
		return
	}
	for _, clause := range stmt.Body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, expr := range cc.List {
			if _, ok := nanValue(pass, expr); !ok {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:     expr.Pos(),
				End:     expr.End(),
				Message: "no value equals NaN; this case never matches",
			})
		}
	}
}

// diagnostic returns the report for a comparison with math.NaN(). The
// comparison is always false, except for !=, which is always true.
func diagnostic(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, x, nan ast.Expr) analysis.Diagnostic {
	var (
		message = diagnosticMessage(bin.Op)
		diag    = analysis.Diagnostic{
			Pos:     bin.Pos(),
			End:     bin.End(),
			Message: message,
		}
		text, ok = replacement(pass, file, bin, x, nan)
	)
	if !ok {
		return diag
	}
	diag.Message += "; write " + text
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Write " + text + " (changes behavior)",
		TextEdits: []analysis.TextEdit{{
			Pos:     bin.Pos(),
			End:     bin.End(),
			NewText: []byte(text),
		}},
	}}
	return diag
}

// diagnosticMessage returns the message for a comparison with op.
func diagnosticMessage(op token.Token) string {
	switch op {
	case token.NEQ:
		return "NaN is unequal to every value; this is always true"
	case token.EQL:
		return "no value equals NaN; this is always false"
	default:
		return "NaN is unordered against every value; this is always false"
	}
}

// replacement returns the text that tests x for a NaN, and reports false when
// the comparison cannot be rewritten.
func replacement(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, x, nan ast.Expr) (string, bool) {
	if x == nil || bin.Op != token.EQL && bin.Op != token.NEQ {
		return "", false
	}
	if !oneLine(pass.Fset, bin.Pos(), bin.End()) {
		return "", false
	}
	if commented(file, bin.Pos(), bin.End()) {
		return "", false
	}
	qual, ok := qualifier(pass, nan)
	if !ok {
		return "", false
	}
	arg, ok := argument(pass, x)
	if !ok {
		return "", false
	}
	text := qual + "IsNaN(" + arg + ")"
	if bin.Op == token.NEQ {
		text = "!" + text
	}
	if !fits(pass, bin.Pos(), bin.End(), text) {
		return "", false
	}
	return text, true
}

// nanValue returns the call to math.NaN that e is or converts, and reports
// false for any other expression. A conversion to a floating-point type
// preserves a NaN, so float32(math.NaN()) is read as well.
func nanValue(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	e = ast.Unparen(e)
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	if isMathNaN(pass.TypesInfo, call) && len(call.Args) == 0 {
		return e, true
	}
	if len(call.Args) != 1 || !pass.TypesInfo.Types[call.Fun].IsType() {
		return nil, false
	}
	t := pass.TypesInfo.TypeOf(call.Fun)
	if t == nil || !isFloat(t) {
		return nil, false
	}
	return nanValue(pass, call.Args[0])
}

// isMathNaN reports whether call is a call to math.NaN.
func isMathNaN(info *types.Info, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(info, call)
	return fn != nil && fn.Name() == "NaN" && fn.Pkg() != nil &&
		fn.Pkg().Path() == "math"
}

// isFloat reports whether t is a floating-point type.
func isFloat(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsFloat != 0
}

// qualifier returns the text that names math at a NaN call, and reports false
// for a file that dot-imports math. A selector is the only form that keeps the
// name the file gives the package.
func qualifier(pass *analysis.Pass, e ast.Expr) (string, bool) {
	e = ast.Unparen(e)
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return "", false
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "math" {
		return "", false
	}
	return id.Name + ".", true
}

// argument returns the text of e as the argument of IsNaN, converting a float
// type that is not assignable to float64.
func argument(pass *analysis.Pass, e ast.Expr) (string, bool) {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return "", false
	}
	text, ok := astexpr.OneLine(ast.Unparen(e))
	if !ok {
		return "", false
	}
	if types.AssignableTo(t, types.Typ[types.Float64]) {
		return text, true
	}
	if isFloat(t) && reaches(pass, "float64", e.Pos()) {
		return "float64(" + text + ")", true
	}
	return "", false
}

// reaches reports whether name names the predeclared object of that name at
// pos, so the rewrite may write it.
func reaches(pass *analysis.Pass, name string, pos token.Pos) bool {
	return imports.Lookup(pass, name, pos) == types.Universe.Lookup(name)
}

// fits reports whether text may replace the range from to. The range lies on
// one line, and the rewritten line is within the width unless the line is
// already past it.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	// Written positions, which a //line directive does not move.
	var (
		start = pass.Fset.PositionFor(from, false)
		end   = pass.Fset.PositionFor(to, false)
	)
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

// oneLine reports whether the text between lo and hi lies on one line.
func oneLine(fset *token.FileSet, lo, hi token.Pos) bool {
	return fset.PositionFor(lo, false).Line == fset.PositionFor(hi, false).Line
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
