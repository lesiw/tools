// Package preferdecoderune provides an analysis.Analyzer that reports a string
// converted to a rune slice to read its first rune.
//
// []rune(s)[0] decodes every rune in the string to build a slice, then keeps
// the first one. utf8.DecodeRuneInString(s) decodes that one rune, and returns
// its width in bytes too.
//
// The call yields two values where the index yields one, and on an empty
// string the index panics where the call returns (utf8.RuneError, 0), so no
// fix is offered. Such a slice is not usually a defect, so this is a house
// rule: the suite calls utf8.DecodeRuneInString where it reads one rune. The
// README catalogs every check and exemption, and the deltas from go-critic's
// rule.
//
// The rule is like go-critic's preferDecodeRune. Go issue 45260 is about this
// shape and closed without a fix.
package preferdecoderune

import (
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "preferdecoderune",
	Doc:  "reports a string converted to a rune slice to read its first rune",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			index, ok := n.(*ast.IndexExpr)
			if !ok || !converts(pass, index.X) || !isZero(pass, index.Index) {
				return true
			}
			pass.Reportf(
				index.Pos(),
				"string converted to a rune slice; use utf8.DecodeRuneInString",
			)
			return true
		})
	}
	return nil, nil
}

// converts reports whether e converts a string to a rune slice. A call that
// returns a rune slice is not a conversion, though its type is the same.
func converts(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	if !pass.TypesInfo.Types[call.Fun].IsType() {
		return false
	}
	if !isRuneSlice(pass.TypesInfo.TypeOf(call)) {
		return false
	}
	// A string, or a type parameter whose values are all strings, is what an
	// untyped string constant is assignable to.
	operand := pass.TypesInfo.TypeOf(call.Args[0])
	return types.AssignableTo(types.Typ[types.UntypedString], operand)
}

// isZero reports whether e is the constant zero, the index that reads the
// first rune.
func isZero(pass *analysis.Pass, e ast.Expr) bool {
	value := pass.TypesInfo.Types[e].Value
	return value != nil && constant.Sign(value) == 0
}

// runeSliceSet is the type set of ~[]rune, which a type parameter is checked
// against.
var runeSliceSet = constraint(types.NewSlice(types.Typ[types.Rune]))

// constraint returns the constraint ~t.
func constraint(t types.Type) *types.Interface {
	return types.NewInterfaceType(nil, []types.Type{
		types.NewUnion([]*types.Term{types.NewTerm(true, t)}),
	}).Complete()
}

// isRuneSlice reports whether t is a slice whose element type is rune, or a
// type parameter over ~[]rune.
func isRuneSlice(t types.Type) bool {
	if slice, ok := t.Underlying().(*types.Slice); ok {
		elem, ok := slice.Elem().Underlying().(*types.Basic)
		return ok && elem.Kind() == types.Rune
	}
	return types.Satisfies(t, runeSliceSet)
}
