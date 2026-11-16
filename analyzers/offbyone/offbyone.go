// Package offbyone provides an analysis.Analyzer that reports an index that is
// always out of range, and a slice whose bound is an Index result.
//
// len(x) is the number of elements, so the last one is at len(x)-1. Indexing
// at len(x) panics on every call. The compiler accepts it.
//
// strings.Index and bytes.Index return -1 when the substring is absent. A
// slice whose only bound is that result is out of range then. The bound may be
// the variable a short declaration assigns the result to, or the call itself.
// A use of the result before the slice ends the check. A guarantee on the
// operands, such as an earlier Contains, is outside what the check sees.
//
// Like go-critic's offBy1.
package offbyone

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

const sliceMessage = "Index can return -1; the bound may need one added"

var Analyzer = &analysis.Analyzer{
	Name: "offbyone",
	Doc:  "reports an index or slice bound that may be out of range",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.IndexExpr:
				if pastEnd(pass, n) {
					// A fix would guess which index was meant, and
					// the last one is only sometimes right.
					pass.Reportf(n.Pos(), "index out of range")
				}
			case *ast.SliceExpr:
				if indexBound(pass, n) {
					pass.Reportf(n.Pos(), sliceMessage)
				}
			case *ast.BlockStmt:
				checkBlock(pass, n.List)
			case *ast.CaseClause:
				checkBlock(pass, n.Body)
			case *ast.CommClause:
				checkBlock(pass, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// pastEnd reports whether e is x[len(x)].
func pastEnd(pass *analysis.Pass, e *ast.IndexExpr) bool {
	slice, ok := e.X.(*ast.Ident)
	if !ok {
		return false
	}
	call, ok := e.Index.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("len") {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(arg) == pass.TypesInfo.ObjectOf(slice)
}

// checkBlock reports the slices in list that start or end at an Index result
// assigned earlier in the same block, until a statement reads the result for
// anything else.
func checkBlock(pass *analysis.Pass, list []ast.Stmt) {
	for i, stmt := range list {
		ident, value, ok := indexAssign(pass, stmt)
		if !ok {
			continue
		}
		for _, later := range list[i+1:] {
			if _, ok := later.(*ast.LabeledStmt); ok {
				break
			}
			slices := slicesAt(pass, later, value, ident)
			if len(slices) == 0 {
				if astexpr.Mentions(pass, later, ident) {
					break
				}
				continue
			}
			if usesOther(pass, later, ident, slices) {
				break
			}
			for _, slice := range slices {
				pass.Reportf(slice.Pos(), sliceMessage)
			}
		}
	}
}

// indexAssign returns the variable and the searched value of an assignment
// from strings.Index or bytes.Index.
func indexAssign(pass *analysis.Pass, stmt ast.Stmt) (*ast.Ident, ast.Expr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || (assign.Tok != token.DEFINE && assign.Tok != token.ASSIGN) {
		return nil, nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	ident, ok := ast.Unparen(assign.Lhs[0]).(*ast.Ident)
	if !ok || ident.Name == "_" {
		return nil, nil, false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || !isIndex(pass, call) {
		return nil, nil, false
	}
	return ident, call.Args[0], true
}

// slicesAt returns the slices of value whose only bound is ident.
func slicesAt(pass *analysis.Pass, node ast.Node, value, ident ast.Expr) []*ast.SliceExpr {
	var found []*ast.SliceExpr
	sameBlock(node, func(n ast.Node) bool {
		slice, ok := n.(*ast.SliceExpr)
		if !ok || slice.Slice3 {
			return true
		}
		bound := slice.Low
		if bound == nil {
			bound = slice.High
		} else if slice.High != nil {
			return true
		}
		if bound == nil || !astexpr.Same(pass, bound, ident) {
			return true
		}
		if !astexpr.Same(pass, slice.X, value) {
			return true
		}
		found = append(found, slice)
		return false
	})
	return found
}

// sameBlock visits node without entering a nested statement list, where a
// slice is no longer in the assignment's block.
func sameBlock(node ast.Node, visit func(ast.Node) bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
			return false
		}
		return visit(n)
	})
}

// usesOther reports whether node reads ident outside the slices found.
func usesOther(pass *analysis.Pass, node ast.Node, ident *ast.Ident, slices []*ast.SliceExpr) bool {
	skip := make(map[*ast.SliceExpr]struct{}, len(slices))
	for _, slice := range slices {
		skip[slice] = struct{}{}
	}
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if slice, ok := n.(*ast.SliceExpr); ok {
			if _, found := skip[slice]; found {
				return false
			}
		}
		if expr, ok := n.(ast.Expr); ok && astexpr.Same(pass, expr, ident) {
			found = true
			return false
		}
		return !found
	})
	return found
}

// indexBound reports whether slice has an Index result as the bound at its
// open end.
func indexBound(pass *analysis.Pass, slice *ast.SliceExpr) bool {
	if slice.Slice3 || (slice.Low == nil) == (slice.High == nil) {
		return false
	}
	bound := slice.Low
	if bound == nil {
		bound = slice.High
	}
	call, ok := ast.Unparen(bound).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || !isIndex(pass, call) {
		return false
	}
	return astexpr.Same(pass, call.Args[0], slice.X)
}

// isIndex reports whether call is strings.Index or bytes.Index.
func isIndex(pass *analysis.Pass, call *ast.CallExpr) bool {
	var fn *types.Func
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		fn, _ = pass.TypesInfo.Uses[fun].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = pass.TypesInfo.Uses[fun.Sel].(*types.Func)
	}
	if fn == nil || fn.Name() != "Index" || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "bytes", "strings":
		return true
	}
	return false
}
