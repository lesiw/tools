package unwrap

import (
	"go/ast"
	"go/token"
)

// multiLineLits returns every composite literal that spans more than one line.
func multiLineLits(tokFile *token.File, file *ast.File) []*ast.CompositeLit {
	var lits []*ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !lit.Lbrace.IsValid() || !lit.Rbrace.IsValid() {
			return true
		}
		if !sameLine(tokFile, lit.Lbrace, lit.Rbrace) {
			lits = append(lits, lit)
		}
		return true
	})
	return lits
}

// nestedInLit reports whether pos falls inside a literal in lits. The literal
// whose opening brace is pos does not count. A literal's Pos is its type, so
// the brace is what identifies it.
func nestedInLit(lits []*ast.CompositeLit, pos token.Pos) bool {
	for _, lit := range lits {
		if lit.Lbrace == pos {
			continue // skip self
		}
		if pos >= lit.Pos() && pos <= lit.End() {
			return true
		}
	}
	return false
}
