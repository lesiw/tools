// Package timenowloop provides an analysis.Analyzer that reports a time.Now
// call in the condition of a for statement.
//
// The spec says of the for statement: "The condition is evaluated before each
// iteration" (https://go.dev/ref/spec#For_statements). A time.Now call there
// can run on each iteration.
//
// The README catalogs every exemption.
//
// There is no fix. Moving the call ahead of the loop changes when it runs.
package timenowloop

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "timenowloop",
	Doc:  "reports a time.Now call in a for condition",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			loop, ok := n.(*ast.ForStmt)
			if !ok || loop.Cond == nil {
				return true
			}
			checkCond(pass, loop.Cond)
			return true
		})
	}
	return nil, nil
}

// checkCond reports every time.Now call in cond that runs with it. A nested
// loop is examined on its own, and a function literal only when the
// condition calls it in place.
func checkCond(pass *analysis.Pass, cond ast.Expr) {
	var stack []ast.Node
	ast.Inspect(cond, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		switch n := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			// A nested loop is examined separately.
			return false
		case *ast.FuncLit:
			if !calledInPlace(stack, n) {
				return false
			}
		}
		var call *ast.CallExpr
		if c, ok := n.(*ast.CallExpr); ok {
			if isTimeNow(pass, c) && !atGoCall(stack) {
				call = c
			}
		}
		stack = append(stack, n)
		if call == nil {
			return true
		}
		pass.Report(analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "time.Now is called in the loop condition",
		})
		return true
	})
}

// atGoCall reports whether the node whose ancestors are stack is the call of a
// go statement.
func atGoCall(stack []ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	_, ok := stack[len(stack)-1].(*ast.GoStmt)
	return ok
}

// calledInPlace reports whether stack shows lit as the function of the call
// that encloses it. A literal started with go is not part of the condition's
// evaluation, so it does not count.
func calledInPlace(stack []ast.Node, lit *ast.FuncLit) bool {
	for _, ancestor := range slices.Backward(stack) {
		switch parent := ancestor.(type) {
		case *ast.ParenExpr:
			if ast.Unparen(parent) != lit {
				return false
			}
		case *ast.CallExpr:
			if ast.Unparen(parent.Fun) != lit {
				return false
			}
			return !startedWithGo(stack, parent)
		default:
			return false
		}
	}
	return false
}

// startedWithGo reports whether call is the call of a go statement.
func startedWithGo(stack []ast.Node, call *ast.CallExpr) bool {
	var found bool
	for _, ancestor := range slices.Backward(stack) {
		if !found {
			found = ancestor == call
			continue
		}
		switch parent := ancestor.(type) {
		case *ast.ParenExpr:
			if ast.Unparen(parent) == call {
				continue
			}
			return false
		case *ast.GoStmt:
			return true
		default:
			return false
		}
	}
	return false
}

// isTimeNow matches the package function, so a local name or a method does not
// qualify.
func isTimeNow(pass *analysis.Pass, call *ast.CallExpr) bool {
	var name *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		name = fun.Sel
	case *ast.Ident:
		name = fun
	default:
		return false
	}
	fn, ok := pass.TypesInfo.ObjectOf(name).(*types.Func)
	if !ok {
		return false
	}
	return fn.FullName() == "time.Now"
}
