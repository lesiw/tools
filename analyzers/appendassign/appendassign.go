// Package appendassign provides an analysis.Analyzer that reports an append
// result assigned to another slice that the function still reads.
//
// ys = append(xs, v) leaves xs at its old length, so a function that reads xs
// afterwards is reading a slice the append did not reach.
//
// Report only: neither xs = append(xs, v) nor ys = append(ys, v) preserves the
// behavior, so no fix is offered.
//
// The rule is adapted from go-critic's appendAssign, which reports the
// assignment without the later read and takes a field on either side.
package appendassign

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "appendassign",
	Doc:  "reports an append result assigned to another slice the function still reads",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if assign, ok := n.(*ast.AssignStmt); ok {
				check(pass, within(stack), assign)
			}
			stack = append(stack, n)
			return true
		})
	}
	return nil, nil
}

// within returns the body of the innermost function the stack is inside.
func within(stack []ast.Node) *ast.BlockStmt {
	for _, n := range slices.Backward(stack) {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			return fn.Body
		case *ast.FuncLit:
			return fn.Body
		}
	}
	return nil
}

// check reports each left side in assign that appends to one slice and assigns
// the result to another the body reads again.
func check(pass *analysis.Pass, body *ast.BlockStmt, assign *ast.AssignStmt) {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for i, rhs := range assign.Rhs {
		dst, ok := ident(assign.Lhs[i])
		if !ok {
			continue
		}
		call, ok := appendCall(pass, rhs)
		if !ok {
			continue
		}
		src, ok := ident(call.Args[0])
		if !ok || src.Name == dst.Name {
			continue
		}
		if !readAgain(pass, body, rhs.End(), pass.TypesInfo.ObjectOf(src)) {
			continue
		}
		pass.Reportf(
			rhs.Pos(),
			"append to %s is assigned to %s, so %s keeps its length",
			src.Name, dst.Name, src.Name,
		)
	}
}

// ident returns e as an identifier that names something other than the blank.
func ident(e ast.Expr) (*ast.Ident, bool) {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name == "_" {
		return nil, false
	}
	return id, true
}

// readAgain reports whether body reads obj after pos.
func readAgain(pass *analysis.Pass, body *ast.BlockStmt, pos token.Pos, obj types.Object) bool {
	var (
		found bool
		stack []ast.Node
	)
	// later reports whether id names obj after pos without being a target.
	later := func(id *ast.Ident) bool {
		return id.Pos() > pos && !written(stack, id) &&
			pass.TypesInfo.ObjectOf(id) == obj
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if id, ok := n.(*ast.Ident); ok && later(id) {
			found = true
			return false
		}
		stack = append(stack, n)
		return true
	})
	return found
}

// written reports whether id is a target of the statement that holds it.
func written(stack []ast.Node, id *ast.Ident) bool {
	if len(stack) == 0 {
		return false
	}
	switch stmt := stack[len(stack)-1].(type) {
	case *ast.AssignStmt:
		return slices.Contains(stmt.Lhs, ast.Expr(id))
	case *ast.RangeStmt:
		return stmt.Key == ast.Expr(id) || stmt.Value == ast.Expr(id)
	}
	return false
}

// appendCall returns e as a call to the builtin append that adds values. A
// spread append concatenates two slices the function already has, so a third
// slice taking the result is deliberate.
func appendCall(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || call.Ellipsis != token.NoPos {
		return nil, false
	}
	builtin, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[builtin] != types.Universe.Lookup("append") {
		return nil, false
	}
	return call, true
}
