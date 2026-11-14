// Package truncatecmp provides an analysis.Analyzer that reports a comparison
// through a narrowing integer conversion.
//
// The specification says an integer conversion is truncated to fit in the
// result type's size, so a comparison through one compares the truncated
// value:
//
//	func below(wide int64, bound int32) bool {
//		return int32(wide) < bound
//	}
//
// For wide holding 1<<32, int32(wide) is 0, and the tool cannot tell whether
// that was meant, so the rule reports without a fix. The README catalogs the
// check and its exemptions.
//
// The rule is adapted from go-critic's truncateCmp, which reads six named
// conversion targets where this rule reads every integer type.
package truncatecmp

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "truncatecmp",
	Doc:  "reports a comparison through a narrowing integer conversion",
	Run:  run,
}

// truncation is a conversion in a comparison that drops bits.
type truncation struct {
	expr *ast.CallExpr
	from types.Type
	to   types.Type
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if bin, ok := n.(*ast.BinaryExpr); ok {
				check(pass, bin)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the one operand of a comparison that a narrowing conversion
// truncates.
func check(pass *analysis.Pass, bin *ast.BinaryExpr) {
	switch bin.Op {
	case token.EQL, token.GEQ, token.GTR, token.LEQ, token.LSS, token.NEQ:
	default:
		return
	}
	t, ok := truncationAt(pass, bin.X)
	if !ok {
		t, ok = truncationAt(pass, bin.Y)
	}
	if !ok {
		return
	}
	from := types.TypeString(t.from, qualifier(pass))
	to := types.TypeString(t.to, qualifier(pass))
	pass.Reportf(t.expr.Pos(), "comparison truncates %s to %s", from, to)
}

// qualifier names a type from another package by its package name, so a
// message reads tracev2.ProcStatus rather than the import path.
func qualifier(pass *analysis.Pass) types.Qualifier {
	return func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}
		return p.Name()
	}
}

// truncationAt returns the conversion to a narrower integer type at the top of
// e, if it has one.
func truncationAt(pass *analysis.Pass, e ast.Expr) (truncation, bool) {
	conv, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return truncation{}, false
	}
	// Without this, a call to a function that returns an integer type reads as
	// a conversion to that type.
	if tv := pass.TypesInfo.Types[conv.Fun]; !tv.IsType() {
		return truncation{}, false
	}
	// A constant has to be representable in the type it is converted to, so
	// the compiler has already refused any conversion that would drop bits.
	if pass.TypesInfo.Types[conv.Args[0]].Value != nil {
		return truncation{}, false
	}
	from := pass.TypesInfo.TypeOf(conv.Args[0])
	to := pass.TypesInfo.TypeOf(conv)
	if !integer(from) || !integer(to) {
		return truncation{}, false
	}
	if pass.TypesSizes.Sizeof(from) <= pass.TypesSizes.Sizeof(to) {
		return truncation{}, false
	}
	return truncation{expr: conv, from: from, to: to}, true
}

// integer reports whether t has an integer underlying type. A type parameter
// does not, which also keeps Sizes.Sizeof from being called with one: it
// asserts on a type parameter.
func integer(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}
