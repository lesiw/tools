// Package breakinselect provides an analysis.Analyzer that reports a break at
// the end of a select case in a loop when it leaves only the select.
//
// A break inside a select leaves the select. A break at the end of a case that
// leaves only the select has no effect, because the select ends either way. A
// labeled break whose label names a loop, switch, or another select is not
// reported. Execution continues after the select, and a break meant to leave
// the loop does not leave it.
//
// Report only: the repair is a label on the loop or a return, and the tool
// cannot determine which one the author meant.
//
// Adapted from staticcheck's SA4011.
package breakinselect

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "breakinselect",
	Doc:  "reports a break at the end of a select case in a loop when it leaves only the select",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		labelOf := make(map[ast.Stmt]types.Object)
		ast.Inspect(file, func(n ast.Node) bool {
			if labeled, ok := n.(*ast.LabeledStmt); ok {
				labelOf[labeled.Stmt] = pass.TypesInfo.Defs[labeled.Label]
			}
			return true
		})
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if sel, ok := n.(*ast.SelectStmt); ok && inLoop(stack) {
				check(pass, sel, labelOf[sel])
			}
			stack = append(stack, n)
			return true
		})
	}
	return nil, nil
}

// inLoop reports whether the stack holds a loop before a function boundary.
func inLoop(stack []ast.Node) bool {
	for _, n := range slices.Backward(stack) {
		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return false
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}

// check reports the break at the end of each case in sel. The label is the one
// on sel, or nil when sel has none.
func check(pass *analysis.Pass, sel *ast.SelectStmt, label types.Object) {
	for _, stmt := range sel.Body.List {
		clause, ok := stmt.(*ast.CommClause)
		if !ok {
			continue
		}
		for _, branch := range tailBreaks(pass, clause.Body, label) {
			pass.Reportf(
				branch.Pos(),
				"break at the end of a select case has no effect",
			)
		}
	}
}

// tailBreaks returns the breaks that end body and leave the select.
func tailBreaks(pass *analysis.Pass, body []ast.Stmt, label types.Object) []*ast.BranchStmt {
	if len(body) == 0 {
		return nil
	}
	return tailBreak(pass, body[len(body)-1], label)
}

// tailBreak returns the breaks that end stmt and leave the select. A break
// inside a nested for, switch, or select belongs to that statement, so the
// check stops there. A labeled break is reported only when its label names the
// select being checked.
func tailBreak(pass *analysis.Pass, stmt ast.Stmt, label types.Object) []*ast.BranchStmt {
	switch stmt := stmt.(type) {
	case *ast.BranchStmt:
		if stmt.Tok != token.BREAK {
			break
		}
		if stmt.Label == nil || pass.TypesInfo.Uses[stmt.Label] == label {
			return []*ast.BranchStmt{stmt}
		}
	case *ast.BlockStmt:
		return tailBreaks(pass, stmt.List, label)
	case *ast.LabeledStmt:
		return tailBreak(pass, stmt.Stmt, label)
	case *ast.IfStmt:
		out := tailBreaks(pass, stmt.Body.List, label)
		switch branch := stmt.Else.(type) {
		case *ast.BlockStmt:
			out = append(out, tailBreaks(pass, branch.List, label)...)
		case *ast.IfStmt:
			out = append(out, tailBreak(pass, branch, label)...)
		}
		return out
	}
	return nil
}
