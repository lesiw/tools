// Package redundantsprint provides an analysis.Analyzer that reports a
// fmt.Sprint whose operand is already a string.
//
// The fmt package doc gives %v as "the value in a default format", and a
// string formats as itself, so a call on one string returns it unchanged. A
// conversion of a string to string returns the same value, so a chain is
// redundant as well: string(fmt.Sprint(s)) becomes s.
//
// The fix drops the wrappers and leaves the operand. It adds a pair of
// parentheses when the wrapper's value is indexed or sliced and the operand is
// a binary, unary, or pointer expression: fmt.Sprint(a + b)[0] becomes
// (a + b)[0]. A comment in the dropped text withholds the fix. The README
// lists the cases this rule leaves alone.
//
// The rule is adapted from go-critic's redundantSprint, which also rewrites a
// fmt.Sprint of a fmt.Stringer to a call of its String method.
package redundantsprint

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "redundantsprint",
	Doc:  "reports a fmt.Sprint whose operand is already a string",
	Run:  run,
}

// checker holds one file of the package.
type checker struct {
	pass *analysis.Pass
	file *ast.File
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			c     = checker{pass, file}
			stack []ast.Node
		)
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if call, ok := n.(*ast.CallExpr); ok {
				outer, parent := enclosing(stack)
				c.report(parent, outer, call)
			}
			return true
		})
	}
	return nil, nil
}

// enclosing returns the expression on top of stack, the parentheses written
// around it included, and the node that holds that expression.
func enclosing(stack []ast.Node) (ast.Expr, ast.Node) {
	var (
		i     = len(stack) - 1
		outer = stack[i].(ast.Expr)
	)
	for {
		paren, ok := stack[i-1].(*ast.ParenExpr)
		if !ok {
			return outer, stack[i-1]
		}
		outer, i = paren, i-1
	}
}

// report reports the chain of wrappers call starts, unless its operand cannot
// stand where the chain stands or an outer wrapper of the chain reports it.
func (c checker) report(parent ast.Node, outer ast.Expr, call *ast.CallExpr) {
	if c.wrapped(parent, call) {
		return
	}
	expr, hasSprint := c.innermost(call)
	if !hasSprint || c.requiresCall(parent, expr) {
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:            outer.Pos(),
		End:            outer.End(),
		Message:        "redundant wrapper; the operand is already a string",
		SuggestedFixes: c.fix(parent, outer, expr),
	})
}

// wrapped reports whether an enclosing call returns call unchanged. That call
// carries the finding, so a chain reports once.
func (c checker) wrapped(parent ast.Node, call *ast.CallExpr) bool {
	outer, ok := parent.(*ast.CallExpr)
	if !ok {
		return false
	}
	value, _, ok := c.unwrap(outer)
	return ok && ast.Unparen(value) == call
}

// innermost returns the value a chain of wrappers leaves unchanged, and
// whether the chain holds a fmt.Sprint.
func (c checker) innermost(call *ast.CallExpr) (ast.Expr, bool) {
	var (
		value     ast.Expr = call
		hasSprint bool
	)
	for {
		next, sprint, ok := c.unwrap(value)
		if !ok {
			break
		}
		value, hasSprint = next, hasSprint || sprint
	}
	return value, hasSprint
}

// requiresCall reports whether the position needs a function call and expr is
// not one. An expression statement, a go, and a defer all take a function
// call. A conversion and a builtin are shaped like one without being one, and
// the type checker has already resolved which of the three call.Fun is.
func (c checker) requiresCall(parent ast.Node, expr ast.Expr) bool {
	switch parent.(type) {
	case *ast.GoStmt, *ast.DeferStmt, *ast.ExprStmt:
	default:
		return false
	}
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return true
	}
	tv := c.pass.TypesInfo.Types[call.Fun]
	return tv.IsType() || tv.IsBuiltin()
}

// unwrap returns the value a call leaves unchanged, and whether a fmt.Sprint
// produced it. A conversion of a string to string and a fmt.Sprint of one
// string are the only wrappers, so the operand of both is a string.
func (c checker) unwrap(expr ast.Expr) (value ast.Expr, sprint, ok bool) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false, false
	}
	if !plainString(c.pass.TypesInfo.TypeOf(call.Args[0])) {
		return nil, false, false
	}
	if c.sprintCall(call) {
		return call.Args[0], true, true
	}
	return call.Args[0], false, c.converted(call)
}

// sprintCall reports whether the callee of call is fmt.Sprint.
func (c checker) sprintCall(call *ast.CallExpr) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprint" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := c.pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == "fmt"
}

// converted reports whether call is a conversion to string. A conversion to a
// named type is not a wrapper: dropping it would change the result's type.
func (c checker) converted(call *ast.CallExpr) bool {
	tv, ok := c.pass.TypesInfo.Types[call.Fun]
	return ok && tv.IsType() && plainString(tv.Type)
}

// plainString reports whether t, with any alias resolved, is the builtin
// string or an untyped string. A named string type is left out: it may have a
// String method, which Sprint would call, and it cannot stand where the
// wrapper returned a string.
func plainString(t types.Type) bool {
	basic, ok := types.Unalias(t).(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// fix returns the edits that drop the wrappers around expr, leaving it in
// place, and nothing when the edit would delete a comment, because that
// comment belongs to the author.
func (c checker) fix(parent ast.Node, outer, expr ast.Expr) []analysis.SuggestedFix {
	expr = ast.Unparen(expr)
	if commented(c.file, outer.Pos(), expr.Pos()) {
		return nil
	}
	if commented(c.file, expr.End(), outer.End()) {
		return nil
	}
	var (
		left  = analysis.TextEdit{Pos: outer.Pos(), End: expr.Pos()}
		right = analysis.TextEdit{Pos: expr.End(), End: outer.End()}
	)
	if needsParens(parent, outer, expr) {
		left.NewText = []byte("(")
		right.NewText = []byte(")")
	}
	return []analysis.SuggestedFix{{
		Message:   "Drop the wrapper",
		TextEdits: []analysis.TextEdit{left, right},
	}}
}

// needsParens reports whether expr needs a pair of parentheses when it stands
// where outer stands in parent. An index or a slice binds into a binary,
// unary, or pointer expression, and a string has no fields or methods, is not
// an interface, and is not callable, so those are the positions that would
// bind into the operand.
func needsParens(parent ast.Node, outer, expr ast.Expr) bool {
	switch parent := parent.(type) {
	case *ast.IndexExpr:
		if parent.X != outer {
			return false
		}
	case *ast.SliceExpr:
		if parent.X != outer {
			return false
		}
	default:
		return false
	}
	switch expr.(type) {
	case *ast.BinaryExpr, *ast.UnaryExpr, *ast.StarExpr:
		return true
	}
	return false
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
