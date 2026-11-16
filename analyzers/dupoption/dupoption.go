// Package dupoption provides an analysis.Analyzer that reports an option that
// a later argument repeats.
//
// A call that hands one option twice reads as if two different options were
// meant. What the second copy does depends on the option: it overwrites the
// first assignment, or it adds a second entry.
//
// The rule is adapted from go-critic's dupOption, which compares only the
// printed text of each argument. The README catalogs every check and
// exemption.
//
// There is no fix: the tool cannot see what the option does, so removing one
// of the two could change behavior.
package dupoption

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupoption",
	Doc:  "reports an option that a later argument repeats",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			opts := options(pass, call)
			for i, opt := range opts {
				if repeated(pass, opt, opts[i+1:]) {
					pass.Report(analysis.Diagnostic{
						Pos:     opt.Pos(),
						End:     opt.End(),
						Message: "argument is repeated later",
					})
				}
			}
			return true
		})
	}
	return nil, nil
}

// options returns the arguments a call passes to a variadic parameter of
// options, or nil for a call without one.
func options(pass *analysis.Pass, call *ast.CallExpr) []ast.Expr {
	if ident, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		// append takes such a tail, but it adds each function to the list
		// instead of applying it.
		if _, ok := pass.TypesInfo.Uses[ident].(*types.Builtin); ok {
			return nil
		}
	}
	typ := pass.TypesInfo.TypeOf(call.Fun)
	if typ == nil {
		return nil
	}
	sign, ok := typ.Underlying().(*types.Signature)
	if !ok || !sign.Variadic() {
		return nil
	}
	last := sign.Params().Len() - 1
	// A multi-value argument can supply every parameter with one expression,
	// as build(settings()) does.
	if last >= len(call.Args) {
		return nil
	}
	// A variadic parameter's type is always a slice of its element.
	tail := sign.Params().At(last).Type().(*types.Slice)
	elem, ok := tail.Elem().Underlying().(*types.Signature)
	if !ok || !isOption(elem) {
		return nil
	}
	return call.Args[last:]
}

// isOption reports whether f is the shape an option takes: it configures the
// value it is handed, so it takes one parameter or more and returns nothing or
// one error. A function that returns a value passes it on, so a repeat of one
// of those composes instead of overwriting.
func isOption(f *types.Signature) bool {
	if f.Params().Len() == 0 {
		return false
	}
	results := f.Results()
	if results.Len() == 0 {
		return true
	}
	errorType := types.Universe.Lookup("error").Type()
	return results.Len() == 1 &&
		types.Identical(results.At(0).Type(), errorType)
}

// repeated reports whether rest holds the same option as opt.
func repeated(pass *analysis.Pass, opt ast.Expr, rest []ast.Expr) bool {
	return slices.ContainsFunc(rest, func(other ast.Expr) bool {
		return sameOption(pass, opt, other)
	})
}

// sameOption reports whether two arguments are the same option: the same
// expression, or the same function applied to the same arguments.
func sameOption(pass *analysis.Pass, a, b ast.Expr) bool {
	var (
		first, firstIsCall   = ast.Unparen(a).(*ast.CallExpr)
		second, secondIsCall = ast.Unparen(b).(*ast.CallExpr)
	)
	if !firstIsCall || !secondIsCall {
		// A literal is a new value each time, so two of them are two
		// functions rather than one option named twice.
		if isLiteral(a) || isLiteral(b) {
			return false
		}
		return astexpr.SamePure(pass, a, b)
	}
	if len(first.Args) != len(second.Args) {
		return false
	}
	if first.Ellipsis.IsValid() != second.Ellipsis.IsValid() {
		return false
	}
	if !astexpr.SamePure(pass, first.Fun, second.Fun) {
		return false
	}
	for i := range first.Args {
		if !astexpr.SamePure(pass, first.Args[i], second.Args[i]) {
			return false
		}
	}
	return true
}

// isLiteral reports whether e is a function literal.
func isLiteral(e ast.Expr) bool {
	_, ok := ast.Unparen(e).(*ast.FuncLit)
	return ok
}
