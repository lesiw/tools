package unwrap

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// collapseWrappedLines merges a wrapped binary expression or call. It keeps
// the merge only when the printer and the width rules both allow the line.
func collapseWrappedLines(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file

		sigs    = signatures(file)
		headers = headerSpans(file)
		lits    = multiLineLits(tokFile, file)

		// consider reports whether the merge rules look at this span
		// at all.
		consider = func(pos, end token.Pos) bool {
			if sameLine(tokFile, pos, end) {
				return false
			}
			if inHeader(headers, pos) {
				// A header stays on one line. collapseHeaders
				// owns it.
				return false
			}
			// A nested multi-line composite literal was expanded
			// for readability. A // comment inside the span forces
			// a line break. The width check does not see that
			// break.
			if nestedInLit(lits, pos) || lineCommentWithin(file, pos, end) {
				return false
			}
			// A wrap inside a declared signature is the author's
			// chosen layout.
			return !inSignature(sigs, pos)
		}
	)
	ast.Inspect(file, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if consider(bin.Pos(), bin.End()) {
			join(r, bin.Pos(), bin.End())
		}
		return true
	})
	// A call is measured as the statement it sits in. The left-hand side
	// of an assignment and any chaining then count toward the fit check.
	snap := r.snapshot()
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if !call.Lparen.IsValid() || !call.Rparen.IsValid() {
			return true
		}
		if nestedInLit(lits, call.Pos()) {
			return true
		}
		pos, end := enclosingStmt(stack)
		if !consider(pos, end) {
			return true
		}
		// A merge whose text is already too long stays too long. No
		// width rule accepts it. Asking the printer costs a print of
		// the whole file, so the render the rule started with rejects
		// most of them. A span that render cannot describe goes to the
		// printer.
		if merged, ok := snap.merged(pos, end); ok && !fits(merged) {
			return true
		}
		join(r, pos, end)
		return true
	})
}

// enclosingStmt returns the span of the innermost statement in stack. It
// returns the innermost node when no statement encloses it.
func enclosingStmt(stack []ast.Node) (pos, end token.Pos) {
	for _, node := range slices.Backward(stack) {
		if stmt, ok := node.(ast.Stmt); ok {
			return stmt.Pos(), stmt.End()
		}
	}
	innermost := stack[len(stack)-1]
	return innermost.Pos(), innermost.End()
}

// collapseSingleDecls removes the parentheses from a declaration block that
// holds one spec. const ( X = 1 ) becomes const X = 1. The lines the block
// occupied are merged with it, so the line table keeps counting the lines
// go/printer will emit.
func collapseSingleDecls(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
	)
	ast.Inspect(file, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if !ok || !gd.Lparen.IsValid() || len(gd.Specs) != 1 {
			return true
		}
		if commentWithin(file, gd.Lparen, gd.Rparen) {
			return true
		}
		lparen, rparen := gd.Lparen, gd.Rparen
		before := slices.Clone(tokFile.Lines())
		mergeSpan(tokFile, gd.TokPos, gd.Specs[0].Pos())
		mergeSpan(tokFile, gd.Specs[0].End(), gd.Rparen)
		gd.Lparen, gd.Rparen = token.NoPos, token.NoPos
		// The parentheses stay when the render disagrees. A spec can
		// keep lines of its own, such as a struct type written across
		// lines. They also stay when removing them would make the line
		// too long. `const (` costs a tab. `const ` costs six columns.
		if !r.keeps(before, gd.TokPos, gd.End(), false) {
			gd.Lparen, gd.Rparen = lparen, rparen
		}
		return true
	})
}

// commentWithin reports whether any comment lies between two positions. A pass
// that rewrites the text between two delimiters drops the comment that sits
// there. Comments are never deleted.
func commentWithin(file *ast.File, lo, hi token.Pos) bool {
	for _, cg := range file.Comments {
		if cg.Pos() > lo && cg.Pos() < hi {
			return true
		}
	}
	return false
}

// lineCommentWithin reports whether a // comment lies between two positions. A
// // comment runs to the end of its line. Merging the lines it spans would
// swallow every token that follows it.
func lineCommentWithin(file *ast.File, lo, hi token.Pos) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if c.Slash <= lo || c.Slash >= hi {
				continue
			}
			if strings.HasPrefix(c.Text, "//") {
				return true
			}
		}
	}
	return false
}
