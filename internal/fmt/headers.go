package fmt

import (
	"go/ast"
	"go/token"
)

// headerSpans returns the span of every if, for, range, and switch header. The
// span runs from the keyword through the body's opening brace. A header stays
// on one line or takes several. The layout passes treat the span as atomic.
// They never wrap into it and never expand it. They collapse it when the whole
// header fits.
func headerSpans(file *ast.File) [][2]token.Pos {
	var spans [][2]token.Pos
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

// inHeader reports whether pos falls inside any header span.
func inHeader(spans [][2]token.Pos, pos token.Pos) bool {
	for _, s := range spans {
		if pos >= s[0] && pos <= s[1] {
			return true
		}
	}
	return false
}

// collapseHeaders merges every multi-line header that fits within the width
// limit back onto one line.
func collapseHeaders(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
	)
	for _, s := range headerSpans(file) {
		if sameLine(tokFile, s[0], s[1]) {
			continue
		}
		if lineCommentWithin(file, s[0], s[1]) {
			continue
		}
		join(r, s[0], s[1])
	}
}
