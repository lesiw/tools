// Package dupsubexpr provides an analysis.Analyzer that reports an operator
// whose operands are the same value.
//
// items[k] < items[k] compares one item with itself, so one of the two
// operands was meant to name something else. The rule is like go-critic's
// dupSubExpr, with a narrower set of operators and a wider NaN exemption; the
// README catalogs every check and exemption.
//
// The comparison operators section of the spec compares floating point "as
// defined by the IEEE 754 standard". Under that standard a NaN is unequal to
// itself, which is why an operand that can hold one is left alone.
//
// There is no fix: the tool cannot tell which operand was meant.
package dupsubexpr

import (
	"go/ast"
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupsubexpr",
	Doc:  "reports an operator whose operands are the same value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			b, ok := n.(*ast.BinaryExpr)
			if !ok || !repeated(pass, b) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     b.Pos(),
				End:     b.End(),
				Message: "both operands are the same value",
			})
			return false // a repeat inside a repeat is the same typo
		})
	}
	return nil, nil
}

// repeated reports whether bin applies its operator to one expression twice.
func repeated(pass *analysis.Pass, bin *ast.BinaryExpr) bool {
	var nanMatters bool
	switch bin.Op {
	case token.EQL, token.NEQ, token.LEQ, token.GEQ, token.QUO, token.SUB:
		nanMatters = true // a NaN changes each of these answers
	case token.LSS, token.GTR, token.REM:
		// < and > are false whether or not a NaN is involved, and % takes no
		// float.
	default:
		return false
	}
	x, y := ast.Unparen(bin.X), ast.Unparen(bin.Y)
	// Equal text is one expression, so one effect check answers for both.
	if astexpr.Text(x) != astexpr.Text(y) || astexpr.Effectful(pass, x) {
		return false
	}
	if nanMatters {
		// f != f can be a NaN test, and x's type stands for y's. A constant is
		// never a NaN, so 1.0 == 1.0 still reports.
		operand := pass.TypesInfo.Types[x]
		if operand.Value == nil && astexpr.MayBeNaN(operand.Type) {
			return false
		}
	}
	// A constant expression is left alone unless it is a bool: [2 / 2]byte is
	// one byte, and 1 == 1 reports.
	v := pass.TypesInfo.Types[bin].Value
	return v == nil || v.Kind() == constant.Bool
}
