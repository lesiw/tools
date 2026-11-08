// Package linewrap provides an analysis.Analyzer for source lines that
// wrap where they should not.
//
// An if, for, or switch statement keeps its header on a single
// line, meaning the opening brace of the body sits on the same
// physical line as the keyword that owns it. Splitting a condition,
// an init statement, a range expression, or a switch tag pushes the
// brace down and buries the branch point under a block of arguments.
// Hoist the value to a named variable instead:
//
//	if err := longCall( // if header spans 3 lines, exceeds the 1-line limit
//	    ctx, arg,
//	); err != nil {
//
// Only the keyword-to-brace distance is measured: case and else
// bodies are out of scope, as are function signatures. A label
// does not move the report; it lands on the keyword. A trailing
// line comment never affects the verdict, while a block comment
// that pushes the brace to a later line is reported. There is no
// length carve-out: hoist the value to a named variable.
package linewrap

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "linewrap",
	Doc:  "reports statement headers that wrap",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			kind, lbrace := header(n)
			if kind == "" {
				return true
			}
			var (
				keyword = pass.Fset.Position(n.Pos()).Line
				brace   = pass.Fset.Position(lbrace).Line
			)
			if keyword != brace {
				pass.Reportf(n.Pos(), "%s header spans %d lines, exceeds the 1-line limit", kind, brace-keyword+1)
			}
			return true
		})
	}
	return nil, nil
}

func header(n ast.Node) (kind string, lbrace token.Pos) {
	switch node := n.(type) {
	case *ast.IfStmt:
		return "if", node.Body.Lbrace
	case *ast.ForStmt:
		return "for", node.Body.Lbrace
	case *ast.RangeStmt:
		return "for", node.Body.Lbrace
	case *ast.SwitchStmt:
		return "switch", node.Body.Lbrace
	case *ast.TypeSwitchStmt:
		return "switch", node.Body.Lbrace
	}
	return "", token.NoPos
}
