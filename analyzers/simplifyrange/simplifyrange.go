// Package simplifyrange provides an analysis.Analyzer that reports a range
// clause that writes the blank identifier instead of leaving it out.
//
// The section of the spec on for statements says that "if the last iteration
// variable is the blank identifier, the range clause is equivalent to the same
// clause without that identifier" (https://go.dev/ref/spec#For_statements).
// The same section says the range expression is not evaluated when at most one
// iteration variable is present and x or len(x) is constant, so removing the
// blank from a clause over an array or a pointer to an array can skip an
// evaluation. The simplify command of gofmt writes the same clause without the
// blank.
//
// Vendored from gopls' simplifyrange
// (gopls/internal/analysis/simplifyrange, golang.org/x/tools/gopls v0.24.0),
// which is internal and cannot be imported. The rule is upstream's; the code
// is adapted to this repository's helpers.
//
// The fix writes the clause without the blank. It is withheld when the edit
// would delete a comment, and when the clause ranges over an array or a
// pointer to an array and dropping the last variable stops evaluating the
// range expression. Upstream offers the edit in that last case; the diagnostic
// is the same.
package simplifyrange

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "simplifyrange",
	Doc:  "reports a range clause that can be written without the blank",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if loop, ok := n.(*ast.RangeStmt); ok {
				check(pass, file, loop)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a clause whose blank can be left out, and suggests the clause
// without it.
func check(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt) {
	// A blank key leaves no iteration variable at all, so the edit runs from
	// the key to the range keyword. Otherwise only the comma and the blank
	// value go.
	var pos, end token.Pos
	switch {
	case blank(loop.Key) && (loop.Value == nil || blank(loop.Value)):
		pos, end = loop.Key.Pos(), loop.Range
	case blank(loop.Value):
		pos, end = loop.Key.End(), loop.Value.End()
	default:
		return
	}
	diag := analysis.Diagnostic{
		Pos:     pos,
		End:     end,
		Message: "range clause can be written without the blank",
	}
	// The value is the last iteration variable a two-variable clause has, so
	// it is the one whose removal stops the evaluation.
	skips := loop.Value != nil && skipped(pass, loop.X)
	if !skips && !astexpr.Commented(file, pos, end) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Drop the blank",
			TextEdits: []analysis.TextEdit{{Pos: pos, End: end}},
		}}
	}
	pass.Report(diag)
}

// skipped reports whether a clause that keeps at most one iteration variable
// stops evaluating e: len(e) is a constant, and evaluating e can run code or
// panic.
func skipped(pass *analysis.Pass, e ast.Expr) bool {
	return constantLen(pass, e) && !safeArray(pass, e)
}

// constantLen reports whether len(e) is a constant: e has an array or a
// pointer-to-array type and contains no channel receive or call that cannot be
// folded.
func constantLen(pass *analysis.Pass, e ast.Expr) bool {
	if !arrayTyped(pass, e) {
		return false
	}
	constant := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			// A function literal creates a closure; its body runs later.
			return false
		case *ast.CallExpr:
			if !astexpr.Pure(pass, n) && pass.TypesInfo.Types[n].Value == nil {
				constant = false
			}
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				constant = false
			}
		}
		return constant
	})
	return constant
}

// arrayTyped reports whether e has an array type or a pointer to one.
func arrayTyped(pass *analysis.Pass, e ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return true
	}
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		_, ok = ptr.Elem().Underlying().(*types.Array)
		return ok
	}
	_, ok := t.Underlying().(*types.Array)
	return ok
}

// safeArray reports whether evaluating e can neither panic nor produce a side
// effect. It adds two checks that astexpr.Safe leaves out: a method value on a
// nil interface, and a map literal with an unhashable key.
func safeArray(pass *analysis.Pass, e ast.Expr) bool {
	if !astexpr.Safe(pass, e) {
		return false
	}
	safe := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			sel, ok := pass.TypesInfo.Selections[n]
			if !ok || sel.Kind() != types.MethodVal {
				break
			}
			if _, ok := sel.Recv().Underlying().(*types.Interface); ok {
				safe = false
			}
		case *ast.CompositeLit:
			t := pass.TypesInfo.TypeOf(n)
			if t == nil {
				break
			}
			m, ok := t.Underlying().(*types.Map)
			if ok && !astexpr.NoInterface(m.Key()) {
				safe = false
			}
		}
		return safe
	})
	return safe
}

func blank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}
