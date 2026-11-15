// Package badsorting provides an analysis.Analyzer that reports a sort.Slice
// call whose less function does not order the elements by the indexes it was
// given.
//
// sort.Interface's Less receives two element indexes: "Less reports whether
// the element with index i must sort before the element with index j." A
// comparison that reads a single element never reads the second one, so it
// cannot order the pair. A comparison at a constant index reads the same slot
// every time, whichever element the sort has moved into it:
// rows[i].rank < rows[i].rank compares one element with itself and returns
// false for every pair, and rows[0].rank < rows[1].rank compares whatever sits
// at slots 0 and 1.
//
// A fix would guess which pair the author meant, so the analyzer reports
// without one.
//
// The rule is inspired by go-critic's sortSlice, which reads the less function
// of sort.Slice and reports the comparison that does not use the sorted slice.
// The name comes from go-critic's badSorting, which reports
// xs = sort.StringSlice(xs). The README catalogs every check and exemption.
package badsorting

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "badsorting",
	Doc:  "reports a sort.Slice call whose less function does not order the elements by the indexes it was given",
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
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each expression a call's less function returns that does not
// order the elements at the two indexes the sort gave it.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	less, slice, ok := lessOf(pass, call)
	if !ok {
		return
	}
	for _, ret := range returns(less.Body) {
		// sortmismatch reports a comparison that does not mention the slice.
		if !astexpr.Mentions(pass, ret, slice) {
			continue
		}
		if throughCall(pass, ret, slice) {
			continue
		}
		if orders(pass, ret) {
			continue
		}
		pass.Reportf(
			ret.Pos(),
			"the less function does not order %s by the indexes it was given",
			astexpr.Text(slice),
		)
	}
}

// lessOf returns a call's less function and the slice it sorts, and reports
// false for any call that is not a sort of that shape.
func lessOf(pass *analysis.Pass, call *ast.CallExpr) (*ast.FuncLit, ast.Expr, bool) {
	if len(call.Args) != 2 || !isSort(pass, call) {
		return nil, nil, false
	}
	less, ok := ast.Unparen(call.Args[1]).(*ast.FuncLit)
	if !ok {
		return nil, nil, false
	}
	return less, base(pass, call.Args[0]), true
}

// isSort reports whether call is one of the sort package's slice sorts.
func isSort(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.Pkg().Path() != "sort" {
		return false
	}
	_, ok := sliceFuncs[fn.Name()]
	return ok
}

// base returns the value a slice expression, a conversion, or an address
// reads, so a comparison over the slice orders the same elements as one over a
// prefix, a tail, a conversion of its type, or a pointer to it.
func base(pass *analysis.Pass, e ast.Expr) ast.Expr {
	for {
		e = ast.Unparen(e)
		switch n := e.(type) {
		case *ast.SliceExpr:
			e = n.X
		case *ast.UnaryExpr:
			if n.Op != token.AND {
				return e
			}
			e = n.X
		case *ast.CallExpr:
			// A conversion of the slice reads the same elements.
			if !pass.TypesInfo.Types[n.Fun].IsType() {
				return e
			}
			e = n.Args[0]
		default:
			return e
		}
	}
}

// returns returns the expression each return statement in a body returns. A
// return inside a nested function literal is not read.
func returns(body *ast.BlockStmt) []ast.Expr {
	var results []ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(n.Results) == 1 {
				results = append(results, n.Results[0])
			}
		}
		return true
	})
	return results
}

// throughCall reports whether e hands s to a call that astexpr.Pure does not
// clear, where handed finds s in the call's receiver or its arguments. Such a
// call may read the elements without naming them: rows.at(i) reads the element
// at i, so a comparison that reaches the slice through one is left alone.
func throughCall(pass *analysis.Pass, e, s ast.Expr) bool {
	for n := range ast.Preorder(e) {
		call, ok := n.(*ast.CallExpr)
		if !ok || astexpr.Pure(pass, call) {
			continue
		}
		if handed(pass, call, s) {
			return true
		}
	}
	return false
}

// handed reports whether call is given s, as its receiver or as one of its
// arguments.
func handed(pass *analysis.Pass, call *ast.CallExpr, s ast.Expr) bool {
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if astexpr.Same(pass, base(pass, sel.X), s) {
			return true
		}
	}
	for _, arg := range call.Args {
		if astexpr.Same(pass, base(pass, arg), s) {
			return true
		}
	}
	return false
}

// orders reports whether e reads one value at two different indexes that are
// not constant. Values are grouped by their printed form, so two names for one
// slice read as two values.
func orders(pass *analysis.Pass, e ast.Expr) bool {
	var (
		values = make(map[string]map[string]struct{}, 2)
		add    = func(value, index string) {
			if values[value] == nil {
				values[value] = make(map[string]struct{}, 2)
			}
			values[value][index] = struct{}{}
		}
	)
	for n := range ast.Preorder(e) {
		switch n := n.(type) {
		case *ast.IndexExpr:
			if index := position(pass, n.Index); index != "" {
				add(astexpr.Text(base(pass, n.X)), index)
			}
		case *ast.SliceExpr:
			// rows[i:] starts at the element at i.
			if index := position(pass, n.Low); index != "" {
				add(astexpr.Text(base(pass, n.X)), index)
			}
		}
	}
	for _, indexes := range values {
		if len(indexes) > 1 {
			return true
		}
	}
	return false
}

// position returns the text of an index, and "" for an index that is not about
// the pair the comparison was asked about: a constant, or a bound that is not
// there.
func position(pass *analysis.Pass, e ast.Expr) string {
	if e == nil || pass.TypesInfo.Types[e].Value != nil {
		return ""
	}
	return astexpr.Text(ast.Unparen(e))
}
