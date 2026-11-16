// Package randzero provides an analysis.Analyzer that reports a call to
// math/rand or math/rand/v2 whose upper bound is one.
//
// math/rand.Intn returns "a non-negative pseudo-random number in the half-open
// interval [0,n)" (https://pkg.go.dev/math/rand#Intn). A bound of 1 leaves 0
// as the only value, so the call always returns 0. The upper-bound functions
// in math/rand/v2 and the Rand methods in both packages return a value from
// the same interval.
//
// The bound is reported when it is a constant expression whose value is 1, so
// a named constant and an arithmetic expression are both checked. A call
// through a function or method value held in a variable, or through an
// interface method, is left alone, because the analyzer reads the callee of a
// direct call.
//
// Report only: replacing the call with 0 would stop the generator from
// advancing, which changes every later value it produces.
//
// The rule is adapted from staticcheck's SA4030, which matches the literal 1.
// This rule also matches a constant expression whose value is 1. The README
// lists every check and exemption.
package randzero

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "randzero",
	Doc:  "reports a call to math/rand or math/rand/v2 whose upper bound is one",
	Run:  run,
}

// bounds holds the upper-bound function and method names in each package.
var bounds = map[string]map[string]bool{
	"math/rand": {
		"Int31n": true,
		"Int63n": true,
		"Intn":   true,
	},
	"math/rand/v2": {
		"Int32N":  true,
		"Int64N":  true,
		"IntN":    true,
		"N":       true,
		"Uint32N": true,
		"Uint64N": true,
		"UintN":   true,
	},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok {
				checkCall(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// checkCall reports a call with a constant bound of one.
func checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return
	}
	if !bounds[fn.Pkg().Path()][fn.Name()] {
		return
	}
	at := boundArg(pass, call.Fun)
	if at >= len(call.Args) || !isOne(pass, call.Args[at]) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "the upper bound is 1, so this always returns 0",
	})
}

// boundArg returns the argument index of the upper bound. A method expression
// passes the receiver first.
func boundArg(pass *analysis.Pass, fun ast.Expr) int {
	for {
		switch x := ast.Unparen(fun).(type) {
		case *ast.IndexExpr:
			fun = x.X
		case *ast.IndexListExpr:
			fun = x.X
		case *ast.SelectorExpr:
			sel := pass.TypesInfo.Selections[x]
			if sel != nil && sel.Kind() == types.MethodExpr {
				return 1
			}
			return 0
		default:
			return 0
		}
	}
}

// isOne reports whether e is a constant with the value 1.
func isOne(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[ast.Unparen(e)]
	if !ok || tv.Value == nil {
		return false
	}
	value := constant.ToInt(tv.Value)
	return constant.Compare(value, token.EQL, constant.MakeInt64(1))
}
