// Package duplicatedcond provides an analysis.Analyzer that reports an if
// whose condition repeats the condition of the if it is nested in.
//
// The rule reports when the inner if is the first statement of the outer
// body, has no init statement, and the condition reads the same value both
// times. By the if statements section of the spec, the block is executed when
// the condition is true. No statement runs between the two tests, so the inner
// test is true whenever the outer body runs.
//
// A statement before the inner if can change a value the condition reads, an
// init statement can change one before the inner condition is evaluated, and a
// call with an effect or a receive can give a different value the second time.
// The length of a channel can change when another goroutine sends to or
// receives from it, and a type parameter whose type set may hold one has the
// same problem, so those length reads are left alone. An else if is reached
// only after the first condition was false, so it cannot repeat a test that
// passed.
//
// There is no fix: the tool cannot tell which test was meant.
package duplicatedcond

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "duplicatedcond",
	Doc:  "reports an if whose condition repeats the condition of the if it is nested in",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			outer, ok := n.(*ast.IfStmt)
			if ok {
				check(pass, outer)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the condition of outer's first statement when it repeats
// outer's condition.
func check(pass *analysis.Pass, outer *ast.IfStmt) {
	if len(outer.Body.List) == 0 || !stable(pass, outer.Cond) {
		return
	}
	inner, ok := outer.Body.List[0].(*ast.IfStmt)
	if !ok || inner.Init != nil {
		return
	}
	if !sameCondition(outer.Cond, inner.Cond) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     inner.Cond.Pos(),
		End:     inner.Cond.End(),
		Message: "condition repeats the condition of the if it is nested in",
	})
}

// sameCondition reports whether a and b print the same expression.
func sameCondition(a, b ast.Expr) bool {
	return astexpr.Text(ast.Unparen(a)) == astexpr.Text(ast.Unparen(b))
}

// stable reports whether cond reads the same value both times. A channel's
// length can change when another goroutine sends to or receives from it, and a
// type parameter whose type set may hold one has the same problem.
func stable(pass *analysis.Pass, cond ast.Expr) bool {
	if astexpr.Effectful(pass, cond) {
		return false
	}
	var found bool
	ast.Inspect(cond, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			return true
		}
		if pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
			return true
		}
		found = mayBeChan(pass.TypesInfo.TypeOf(call.Args[0]))
		return !found
	})
	return !found
}

// mayBeChan reports whether a value of type t can be a channel. A type
// parameter is left to its constraint.
func mayBeChan(t types.Type) bool {
	if t == nil {
		return true
	}
	if param, ok := t.(*types.TypeParam); ok {
		return constraintChan(param.Constraint())
	}
	_, ok := t.Underlying().(*types.Chan)
	return ok
}

// constraintChan reports whether the type set of c holds a channel. The
// elements of an interface are intersected, so one element without a channel
// settles the constraint.
func constraintChan(c types.Type) bool {
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if mayBeChan(term.Type()) {
				return true
			}
		}
		return false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return mayBeChan(c)
	}
	for embedded := range iface.EmbeddedTypes() {
		if !constraintChan(embedded) {
			return false
		}
	}
	return true
}
