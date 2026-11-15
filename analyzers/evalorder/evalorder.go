// Package evalorder provides an analysis.Analyzer that reports a return that
// reads a value and passes its address to a call.
//
// The "Order of evaluation" section of the spec fixes the order of function
// calls, method calls, receive operations, and binary logical operations while
// the operands of an expression, an assignment, or a return statement are
// evaluated. It leaves the rest of the order unspecified, so in
//
//	return n, add(&n)
//
// n may be read before add is called or after it returns.
//
// There is no fix: hoisting the call and copying the value each settle the
// order, and the tool cannot tell which value was meant.
//
// The rule is adapted from go-critic's evalOrder. The README catalogs every
// check and exemption.
package evalorder

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "evalorder",
	Doc:  "reports a return that reads a value and passes its address to a call",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	// A return inside a function literal is a return of its own, so the walk
	// over a file enters literals.
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if ret, ok := n.(*ast.ReturnStmt); ok {
				report(pass, ret)
			}
			return true
		})
	}
	return nil, nil
}

// report compares the reads of ret with the addresses ret hands to a call.
func report(pass *analysis.Pass, ret *ast.ReturnStmt) {
	var handed []place
	for _, res := range ret.Results {
		handed = append(handed, addresses(pass, res)...)
	}
	if len(handed) == 0 {
		return
	}
	o := spans(pass, ret)
	reported := map[*types.Var]struct{}{} // one report per variable
	for _, res := range ret.Results {
		for _, r := range reads(pass, res) {
			if _, ok := reported[r.v]; ok {
				continue
			}
			written := slices.ContainsFunc(handed, func(w place) bool {
				return mayWrite(w, r) && !o.ordered(w.expr.Pos(), r.expr.Pos())
			})
			if !written {
				continue
			}
			reported[r.v] = struct{}{}
			pass.Reportf(r.expr.Pos(),
				"%s is read before or after a call that may write it",
				types.ExprString(r.expr),
			)
		}
	}
}
