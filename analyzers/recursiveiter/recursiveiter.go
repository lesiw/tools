// Package recursiveiter provides an analysis.Analyzer that reports an iterator
// that ranges over a call to itself.
//
// Vendored from gopls' recursiveiter
// (gopls/internal/analysis/recursiveiter, golang.org/x/tools/gopls v0.24.0),
// which is internal and cannot be imported. gopls reports a result whose type
// is iter.Seq or iter.Seq2 itself; an alias of either is reported here too.
//
// The README lists the shapes the rule leaves alone.
//
// Report only: the fix would add a traversal function to the recursive type,
// and the tool cannot name it.
package recursiveiter

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "recursiveiter",
	Doc:  "reports an iterator that ranges over a call to itself",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, node := range file.Decls {
			decl, ok := node.(*ast.FuncDecl)
			if !ok {
				continue
			}
			check(pass, decl)
		}
	}
	return nil, nil
}

// check reports each range over a call to itself inside a returned literal.
func check(pass *analysis.Pass, decl *ast.FuncDecl) {
	fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok || decl.Body == nil || !returnsIterator(fn) {
		return
	}
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return true
		}
		lit, ok := ast.Unparen(ret.Results[0]).(*ast.FuncLit)
		if !ok {
			return true
		}
		report(pass, lit, fn)
		return false // do not walk the literal twice
	})
}

// report reports each range in lit whose operand calls fn.
func report(pass *analysis.Pass, lit *ast.FuncLit, fn *types.Func) {
	name := fn.Name()
	ast.Inspect(lit, func(n ast.Node) bool {
		rng, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		call, ok := ast.Unparen(rng.X).(*ast.CallExpr)
		if !ok || typeutil.StaticCallee(pass.TypesInfo, call) != fn {
			return true
		}
		pass.Report(analysis.Diagnostic{
			Pos:     rng.Range,
			End:     rng.X.End(),
			Message: "iterator " + name + " ranges over a call to itself",
		})
		return true
	})
}

// returnsIterator reports whether fn's only result is iter.Seq or iter.Seq2.
// An alias is the same type, so it counts; a defined type is a different one,
// so it does not.
func returnsIterator(fn *types.Func) bool {
	results := fn.Signature().Results()
	if results.Len() != 1 {
		return false
	}
	named, ok := types.Unalias(results.At(0).Type()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil || obj.Pkg().Path() != "iter" {
		return false
	}
	return obj.Name() == "Seq" || obj.Name() == "Seq2"
}
