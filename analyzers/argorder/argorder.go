// Package argorder provides an analysis.Analyzer that reports a call whose
// arguments name the parameters in the other order.
//
// strings.Replace documents its order through its parameter names: "the first
// n non-overlapping instances of old replaced by new" tells a reader which of
// the three strings is which. Two adjacent parameters of one type can be
// swapped without a compile error, so the argument names describe which value
// goes where.
//
// The rule is like go-critic's argOrder, which reports a constant written
// where one of the operands belongs; this rule compares the parameter names of
// the call instead.
//
// Report only: swapping the arguments changes behavior, so there is no fix.
// The README catalogs every check, exemption, and limit.
package argorder

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "argorder",
	Doc:  "reports a call whose arguments name the parameters in the other order",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, call *ast.CallExpr) {
	typ := pass.TypesInfo.TypeOf(call.Fun)
	if typ == nil {
		return
	}
	sig, ok := typ.Underlying().(*types.Signature)
	if !ok {
		return
	}
	// A variadic parameter takes any number of arguments, so no single
	// argument sits at its position.
	n := sig.Params().Len()
	if sig.Variadic() {
		n--
	}
	// A call can have fewer argument expressions than parameters, as
	// join(pairOf()) does.
	pairs := min(n, len(call.Args))
	for i := 0; i+1 < pairs; i++ {
		first, second := sig.Params().At(i), sig.Params().At(i+1)
		// A predeclared function's signature is written without names, as a
		// function type's may be, so there is nothing to compare.
		if first.Name() == "" || second.Name() == "" {
			continue
		}
		if !types.Identical(first.Type(), second.Type()) {
			continue
		}
		a, b := name(call.Args[i]), name(call.Args[i+1])
		if a != second.Name() || b != first.Name() {
			continue
		}
		pass.Reportf(call.Args[i].Pos(),
			"arguments are in the order %s, %s; the parameters are %s, %s",
			a, b, first.Name(), second.Name(),
		)
	}
}

// name returns the name an argument is written as, with any parentheses
// dropped.
func name(e ast.Expr) string {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}
