// Package addrnil provides an analysis.Analyzer that reports a comparison of
// an address with nil.
//
// Taking an address never yields nil, so the comparison has the same answer
// whenever it is evaluated. The rule is like staticcheck's SA4022.
package addrnil

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "addrnil",
	Doc:  "reports a comparison of an address with nil that is always the same",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || !constant(pass, bin) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     bin.Pos(),
				End:     bin.End(),
				Message: "an address cannot be nil",
			})
			return true
		})
	}
	return nil, nil
}

// constant reports whether e compares an address with nil in a way whose
// answer cannot change.
func constant(pass *analysis.Pass, e *ast.BinaryExpr) bool {
	if e.Op != token.EQL && e.Op != token.NEQ {
		return false
	}
	if isAddress(e.X) && pass.TypesInfo.Types[e.Y].IsNil() {
		return true
	}
	return isAddress(e.Y) && pass.TypesInfo.Types[e.X].IsNil()
}

// isAddress reports whether e is an address expression.
func isAddress(e ast.Expr) bool {
	unary, ok := ast.Unparen(e).(*ast.UnaryExpr)
	return ok && unary.Op == token.AND
}
