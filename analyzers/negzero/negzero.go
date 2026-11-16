// Package negzero provides an analysis.Analyzer that reports a unary minus on
// a floating-point or complex constant zero.
//
// The Constants section of the spec says numeric constants "represent exact
// values of arbitrary precision and do not overflow. Consequently, there are
// no constants denoting the IEEE 754 negative zero, infinity, and not-a-number
// values" (https://go.dev/ref/spec#Constants). So -0.0 is the constant zero,
// and the minus is not part of its value. Negative zero exists at run time;
// math.Copysign(0, -1) returns one.
//
// The rule reports the minus when the operand's exact value is zero and the
// expression's type is floating-point or complex. The type may be untyped,
// typed, supplied by the context, or a type parameter restricted to those
// types. So const x = -0.0, -float64(0), float64(-0), var f float64 = -0, and
// an explicit conversion of a typed integer constant zero such as
// float64(-typedIntZero) are reported. An integer zero on its own is left
// alone, and so is a tiny nonzero constant that rounds to zero in the context.
//
// The fix removes the minus; the value is unchanged. An author who wanted
// negative zero needs math.Copysign(0, -1); the analyzer cannot write it
// without changing the value.
//
// The rule is adapted from staticcheck's SA4026. The README catalogs every
// check and exemption.
package negzero

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "negzero",
	Doc:  "reports a unary minus on a floating-point or complex constant zero",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.UnaryExpr:
				if n.Op == token.SUB && zero(pass, n) {
					report(pass, n)
				}
			case *ast.CallExpr:
				if unary, ok := conversion(pass, n); ok {
					report(pass, unary)
				}
			}
			return true
		})
	}
	return nil, nil
}

func report(pass *analysis.Pass, unary *ast.UnaryExpr) {
	diag := analysis.Diagnostic{
		Pos:     unary.Pos(),
		End:     unary.End(),
		Message: "a constant cannot be negative zero; this is zero",
	}
	if fixable(pass, unary) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the minus",
			TextEdits: []analysis.TextEdit{edit(pass, unary)},
		}}
	}
	pass.Report(diag)
}

// fixable reports whether the minus can be removed without moving the operand
// to another line. A line break between the minus and the operand can end the
// statement once the minus is gone.
func fixable(pass *analysis.Pass, unary *ast.UnaryExpr) bool {
	return pass.Fset.PositionFor(unary.OpPos, false).Line ==
		pass.Fset.PositionFor(unary.X.Pos(), false).Line
}

// edit returns the edit that removes the minus and the spaces or tabs after
// it. A space takes the minus's place when the text before it cannot separate
// the tokens, so the edit does not join them.
func edit(pass *analysis.Pass, unary *ast.UnaryExpr) analysis.TextEdit {
	var (
		pos  = unary.OpPos
		end  = pos + 1
		text = []byte(" ")
		tf   = pass.Fset.File(pos)
	)
	if tf == nil {
		return analysis.TextEdit{Pos: pos, End: end, NewText: text}
	}
	src, err := pass.ReadFile(tf.Name())
	if err != nil {
		return analysis.TextEdit{Pos: pos, End: end, NewText: text}
	}
	lo := tf.Offset(pos)
	if lo < 0 || lo >= len(src) {
		return analysis.TextEdit{Pos: pos, End: end, NewText: text}
	}
	after := lo + 1
	for after < len(src) && (src[after] == ' ' || src[after] == '\t') {
		after++
	}
	end = pos + token.Pos(after-lo)
	if lo > 0 && separator(src[lo-1]) {
		text = nil
	}
	return analysis.TextEdit{Pos: pos, End: end, NewText: text}
}

// separator reports whether b can stand between the token before the minus
// and the operand once the minus is gone.
func separator(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '(', '[', '{', ',', ';', ':', '=':
		return true
	}
	return false
}

// zero reports whether unary is a floating-point or complex zero. The
// expression has the type of its context, so an integer zero a context
// converts to a float is read here as well. The operand is checked for an
// exact zero, so a tiny nonzero constant that rounds to zero in the context is
// not reported.
func zero(pass *analysis.Pass, unary *ast.UnaryExpr) bool {
	tv, ok := pass.TypesInfo.Types[unary]
	if !ok || !number(tv.Type) {
		return false
	}
	operand, ok := pass.TypesInfo.Types[unary.X]
	return ok && operand.Value != nil && constant.Sign(operand.Value) == 0
}

// conversion returns the unary minus on a typed integer constant zero that
// call converts to a floating-point or complex type. The unary itself has an
// integer type here, so zero does not read it.
func conversion(pass *analysis.Pass, call *ast.CallExpr) (*ast.UnaryExpr, bool) {
	if len(call.Args) != 1 {
		return nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() || !number(tv.Type) {
		return nil, false
	}
	unary, ok := ast.Unparen(call.Args[0]).(*ast.UnaryExpr)
	if !ok || unary.Op != token.SUB || zero(pass, unary) {
		return nil, false
	}
	operand, ok := pass.TypesInfo.Types[unary.X]
	if !ok || operand.Value == nil || constant.Sign(operand.Value) != 0 {
		return nil, false
	}
	return unary, true
}

// numberTypes returns the constraint whose type set holds every floating-point
// and complex type.
func numberTypes() *types.Interface {
	var (
		union = types.NewUnion([]*types.Term{
			types.NewTerm(true, types.Typ[types.Float32]),
			types.NewTerm(true, types.Typ[types.Float64]),
			types.NewTerm(true, types.Typ[types.Complex64]),
			types.NewTerm(true, types.Typ[types.Complex128]),
		})
		iface = types.NewInterfaceType(nil, []types.Type{union})
	)
	iface.Complete()
	return iface
}

// number reports whether t is a floating-point or complex type. A type
// parameter is read through its type set.
func number(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if param, ok := t.(*types.TypeParam); ok {
		return types.Satisfies(param, numberTypes())
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsFloat|types.IsComplex) != 0
}
