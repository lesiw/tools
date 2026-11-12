// Package header finds the header of each if, for, range, and switch.
package header

import (
	"go/ast"
	"go/token"
)

// Spans returns the span of every if, for, range, and switch header. The span
// runs from the keyword through the body's opening brace. A header stays on
// one line or takes several. The layout passes treat the span as atomic: they
// never wrap into it and never expand it.
func Spans(file *ast.File) (spans [][2]token.Pos) {
	ast.Inspect(file, func(n ast.Node) bool {
		var lo, hi token.Pos
		switch node := n.(type) {
		case *ast.IfStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.ForStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.RangeStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.SwitchStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.TypeSwitchStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		default:
			return true
		}
		spans = append(spans, [2]token.Pos{lo, hi})
		return true
	})
	return spans
}

// In reports whether pos falls inside any header span.
func In(spans [][2]token.Pos, pos token.Pos) bool {
	for _, s := range spans {
		if pos >= s[0] && pos <= s[1] {
			return true
		}
	}
	return false
}
