// Package sortmismatch provides an analysis.Analyzer that reports a sort.Slice
// call whose less function does not name the slice the call was given.
//
// sort.Interface's Less receives two element indexes: "Less reports whether
// the element with index i must sort before the element with index j." A
// comparison over another slice orders the elements by values the call does
// not reorder, so the elements and the values that ordered them no longer line
// up.
//
// A fix would change which slice the call sorts, so the analyzer reports
// without a fix.
//
// The rule is adapted from go-critic's sortSlice, which reads sort.Slice and
// sort.SliceStable and reports a body that returns one comparison of the wrong
// values. This rule reads sort.SliceIsSorted too and reads the whole body. The
// README catalogs every check and exemption.
package sortmismatch

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "sortmismatch",
	Doc:  "reports a sort.Slice call whose less function does not name the slice",
	Run:  run,
}

// sliceFuncs take a slice and a less function over its element indexes.
var sliceFuncs = map[string]struct{}{
	"Slice":         {},
	"SliceIsSorted": {},
	"SliceStable":   {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if slice, ok := mismatchOf(pass, call); ok {
					pass.Reportf(
						call.Pos(), "the less function does not name %s",
						types.ExprString(slice),
					)
				}
			}
			return true
		})
	}
	return nil, nil
}

// mismatchOf returns the slice a call sorts when the call's less function does
// not name it, and reports false for any other call.
func mismatchOf(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	if len(call.Args) != 2 || !isSort(pass, call) {
		return nil, false
	}
	less, ok := ast.Unparen(call.Args[1]).(*ast.FuncLit)
	if !ok {
		return nil, false
	}
	slice := base(call.Args[0])
	if !nameable(slice) || !callFree(slice) || !callFree(less.Body) {
		return nil, false
	}
	return slice, !astexpr.Mentions(pass, less.Body, slice)
}

// isSort reports whether call is one of the sort package's slice functions.
func isSort(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "sort" {
		return false
	}
	_, ok := sliceFuncs[fn.Name()]
	return ok
}

// base returns the value a slice expression reads, so a comparison over the
// slice orders the same elements as one over a prefix or tail of it.
func base(e ast.Expr) ast.Expr {
	for {
		e = ast.Unparen(e)
		slice, ok := e.(*ast.SliceExpr)
		if !ok {
			return e
		}
		e = slice.X
	}
}

// nameable reports whether e is a form a comparison can write: an identifier,
// a field, an element, or a dereference.
func nameable(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
		return true
	}
	return false
}

// callFree reports whether e holds no call. A call in the comparison may read
// the slice without naming it, and a call in the slice argument leaves the
// comparison no base to name.
func callFree(e ast.Node) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		if _, is := n.(*ast.CallExpr); is {
			ok = false
		}
		return ok
	})
	return ok
}
