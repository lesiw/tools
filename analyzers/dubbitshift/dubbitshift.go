// Package dubbitshift provides an analysis.Analyzer that reports a shift of a
// fixed-size typed constant by a count at least as large as the type's width,
// or a shift of an unsigned typed constant whose result is zero below that
// width.
//
// The spec says shifts behave as if the left operand is shifted once per
// count, and that there is no upper limit on the count
// (https://go.dev/ref/spec#Shift_operators). Once the count reaches the width
// of the operand, no bit of the original value remains. A signed right shift
// repeats the sign bit, so its result is zero or -1.
//
// The suite's shift pass reports a non-constant operand shifted by a constant
// count at least the width, and skips a constant operand. This rule applies to
// constant operands, including a shift below the width.
//
// An untyped operand has no width, and an expression with a
// platform-dependent type such as int is left alone, so both are skipped. An
// unsigned typed constant with a fixed-size type is reported when its result
// is zero and its operand is not zero.
//
// The fix replaces the shift with its constant result, as a conversion to the
// operand's type. The README catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's SA9006, which reports a shift of a
// fixed-size integer by a count at least as large as the type width.
package dubbitshift

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "dubbitshift",
	Doc:  "reports a shift of a fixed-size typed constant",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			names = imports.Names(pass, file)
			stack []ast.Node
		)
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				return true
			}
			if expr, ok := shiftOf(n); ok {
				check(pass, file, names, stack, expr)
			}
			stack = append(stack, n)
			return true
		})
	}
	return nil, nil
}

// shiftOf returns n as a shift expression, and reports false for any other
// node.
func shiftOf(n ast.Node) (*ast.BinaryExpr, bool) {
	expr, ok := n.(*ast.BinaryExpr)
	if !ok || (expr.Op != token.SHL && expr.Op != token.SHR) {
		return nil, false
	}
	return expr, true
}

// check reports a constant shift that matches either clause of the rule.
func check(pass *analysis.Pass, file *ast.File, names map[string]string, stack []ast.Node, expr *ast.BinaryExpr) {
	message, ok := finding(pass, expr)
	if !ok {
		return
	}
	report(pass, file, names, stack, expr, message)
}

// finding returns the message for expr when it matches either clause of the
// rule.
func finding(pass *analysis.Pass, expr *ast.BinaryExpr) (string, bool) {
	if pass.TypesInfo.Types[expr.X].Value == nil {
		return "", false // the shift pass reports a non-constant operand
	}
	if inheritedIota(pass, expr) {
		return "", false // one expression node stands for every spec
	}
	if platformDependent(pass, expr.X) || platformDependent(pass, expr.Y) {
		return "", false
	}
	count, ok := countValue(pass, expr.Y)
	if !ok {
		return "", false
	}
	width, ok := widthOf(pass, expr.X)
	if !ok {
		return "", false
	}
	if atLeast(count, width) {
		return fmt.Sprintf(
			"shift count %s is at least the %d-bit width of the operand",
			count.ExactString(), width,
		), true
	}
	if !unsigned(pass, expr.X) {
		return "", false
	}
	value, ok := constantValue(pass, expr)
	if !ok || !isZero(value) || operandZero(pass, expr.X) {
		return "", false
	}
	return "constant shift by " + count.ExactString() +
		" moves every set bit out of the operand", true
}

// report reports expr, with the constant result as its fix where the shift has
// one.
func report(pass *analysis.Pass, file *ast.File, names map[string]string, stack []ast.Node, expr *ast.BinaryExpr, message string) {
	diag := analysis.Diagnostic{
		Pos: expr.Pos(), End: expr.End(), Message: message,
	}
	if nestedFix(pass, file, names, stack) {
		pass.Report(diag)
		return
	}
	if commented(file, expr.Pos(), expr.End()) {
		pass.Report(diag)
		return
	}
	text, ok := fixText(pass, names, expr)
	if !ok {
		pass.Report(diag)
		return
	}
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Replace with the constant result",
		TextEdits: []analysis.TextEdit{{
			Pos:     expr.Pos(),
			End:     expr.End(),
			NewText: []byte(text),
		}},
	}}
	pass.Report(diag)
}

// nestedFix reports whether a shift in stack is reported over the text this
// fix would edit.
func nestedFix(pass *analysis.Pass, file *ast.File, names map[string]string, stack []ast.Node) bool {
	for _, n := range stack {
		expr, ok := shiftOf(n)
		if !ok {
			continue
		}
		if _, ok := finding(pass, expr); !ok {
			continue
		}
		if commented(file, expr.Pos(), expr.End()) {
			continue
		}
		if _, ok := fixText(pass, names, expr); ok {
			return true
		}
	}
	return false
}

// fixText returns the text of the fix for expr, when the shift has one.
func fixText(pass *analysis.Pass, names map[string]string, expr *ast.BinaryExpr) (string, bool) {
	if !selfContained(pass, expr.X) || !selfContained(pass, expr.Y) {
		return "", false
	}
	value, ok := constantValue(pass, expr)
	if !ok {
		return "", false
	}
	return constantText(pass, names, expr, value)
}

// selfContained reports whether e names no constant other than iota. A named
// constant can carry a value that was computed for the target.
func selfContained(pass *analysis.Pass, e ast.Expr) bool {
	contained := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			if namesConstant(pass, pass.TypesInfo.Uses[n]) {
				contained = false
			}
		case *ast.SelectorExpr:
			if namesConstant(pass, pass.TypesInfo.Uses[n.Sel]) {
				contained = false
			}
		}
		return contained
	})
	return contained
}

// namesConstant reports whether obj is a constant other than iota.
func namesConstant(_ *analysis.Pass, obj types.Object) bool {
	c, ok := obj.(*types.Const)
	return ok && c != types.Universe.Lookup("iota")
}

// containsIota reports whether e mentions iota.
func containsIota(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if pass.TypesInfo.Uses[ident] == types.Universe.Lookup("iota") {
			found = true
			return false
		}
		return true
	})
	return found
}

// inheritedIota reports whether e uses iota in a const spec whose expression
// list a later spec omits.
func inheritedIota(pass *analysis.Pass, e ast.Expr) bool {
	if !containsIota(pass, e) {
		return false
	}
	for _, file := range pass.Files {
		var inherited bool
		ast.Inspect(file, func(n ast.Node) bool {
			gen, ok := n.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				return true
			}
			for i, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || !containsValue(vs, e) {
					continue
				}
				if i+1 < len(gen.Specs) {
					next, ok := gen.Specs[i+1].(*ast.ValueSpec)
					if ok && len(next.Values) == 0 {
						inherited = true
						return false
					}
				}
			}
			return true
		})
		if inherited {
			return true
		}
	}
	return false
}

// containsValue reports whether e falls inside one of vs's values.
func containsValue(vs *ast.ValueSpec, e ast.Expr) bool {
	for _, v := range vs.Values {
		if v.Pos() <= e.Pos() && e.End() <= v.End() {
			return true
		}
	}
	return false
}

// platformDependent reports whether e contains a value of platform-dependent
// type anywhere in it.
func platformDependent(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		expr, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		t := pass.TypesInfo.TypeOf(expr)
		if t == nil {
			return true
		}
		b, ok := types.Unalias(t).Underlying().(*types.Basic)
		if ok {
			switch b.Kind() {
			case types.Int, types.Uint, types.Uintptr:
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

// unsigned reports whether e has an unsigned integer type.
func unsigned(pass *analysis.Pass, e ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}
	b, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && b.Info()&types.IsUnsigned != 0
}

// countValue returns the constant count of a shift, and reports false for a
// count that is not a non-negative integer constant.
func countValue(pass *analysis.Pass, e ast.Expr) (constant.Value, bool) {
	v := pass.TypesInfo.Types[e].Value
	if v == nil {
		return nil, false
	}
	v = constant.ToInt(v)
	if v.Kind() != constant.Int || constant.Sign(v) < 0 {
		return nil, false
	}
	return v, true
}

// widthOf returns the width of e in bits, and reports false for a value whose
// width is not fixed. The platform-dependent int, uint, and uintptr are left
// out: their width follows the target, and the idiom ^uint(0) >> 63 tests it.
func widthOf(pass *analysis.Pass, e ast.Expr) (int64, bool) {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return 0, false
	}
	b, ok := types.Unalias(t).Underlying().(*types.Basic)
	if !ok {
		return 0, false
	}
	switch b.Kind() {
	case types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint8, types.Uint16, types.Uint32, types.Uint64:
	default:
		return 0, false
	}
	size := pass.TypesSizes.Sizeof(t)
	if size <= 0 {
		return 0, false
	}
	return 8 * size, true
}

// atLeast reports whether count is at least width.
func atLeast(count constant.Value, width int64) bool {
	return constant.Compare(count, token.GEQ, constant.MakeInt64(width))
}

// constantValue returns the value of e when e is a constant expression.
func constantValue(pass *analysis.Pass, e ast.Expr) (constant.Value, bool) {
	v := pass.TypesInfo.Types[e].Value
	return v, v != nil
}

// isZero reports whether v is the integer constant zero.
func isZero(v constant.Value) bool {
	return v.Kind() == constant.Int && constant.Sign(v) == 0
}

// operandZero reports whether e is the integer constant zero.
func operandZero(pass *analysis.Pass, e ast.Expr) bool {
	v := pass.TypesInfo.Types[e].Value
	return v != nil && v.Kind() == constant.Int && constant.Sign(v) == 0
}

// constantText returns the constant value as a conversion to the operand's
// type.
func constantText(pass *analysis.Pass, names map[string]string, e *ast.BinaryExpr, v constant.Value) (string, bool) {
	t := pass.TypesInfo.TypeOf(e.X)
	if t == nil {
		return "", false
	}
	reachable := true
	qualifier := func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}
		name, ok := imports.Qualifier(pass, names, p.Path(), e.Pos())
		if !ok {
			reachable = false
		}
		return name
	}
	text := types.TypeString(t, qualifier)
	if !reachable {
		return "", false
	}
	return text + "(" + v.ExactString() + ")", true
}

// commented reports whether a comment falls inside the text from pos to end.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
