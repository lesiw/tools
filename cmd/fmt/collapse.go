package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// collapseWrappedLines merges every wrapped binary expression and call
// statement that fits back onto one line.
func collapseWrappedLines(tokFile *token.File, file *ast.File, snap *snapshot) {
	var (
		sigs    = funcSignatures(file)
		headers = headerSpans(file)
		lits    = multiLineLits(tokFile, file)
	)
	fits := func(pos, end token.Pos) bool {
		if tokFile.Line(pos) == tokFile.Line(end) {
			return false
		}
		if inHeader(headers, pos) {
			// A header is single-line-or-nothing and
			// collapseHeaders owns it.
			return false
		}
		// A nested multi-line composite literal was intentionally
		// expanded for readability; a // comment inside the span
		// forces a line break the width check cannot see.
		if nestedInLit(lits, pos) || lineCommentWithin(file, pos, end) {
			return false
		}
		// The wrap inside a declared signature is the author's
		// chosen layout.
		if inFuncSignature(sigs, pos) {
			return false
		}
		if spansDeclGroup(file, pos, end) {
			return false
		}
		w, ok := snap.mergedWidth(pos, end)
		return ok && w <= cfg.Len
	}
	ast.Inspect(file, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if fits(bin.Pos(), bin.End()) {
			mergeSpan(tokFile, bin.Pos(), bin.End())
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if !call.Lparen.IsValid() || !call.Rparen.IsValid() {
			return true
		}
		// Bound the candidate by the smallest enclosing statement so
		// LHS assignments and post-call chaining count toward the
		// line-fit check.
		pos, end := enclosingStmtBounds(file, call)
		if nestedInLit(lits, call.Pos()) {
			return true
		}
		if fits(pos, end) {
			mergeSpan(tokFile, pos, end)
		}
		return true
	})
}

// enclosingStmtBounds returns the start/end positions of the
// smallest [ast.Stmt] enclosing target, or target's own bounds when
// no statement encloses it.
func enclosingStmtBounds(file *ast.File, target ast.Node) (pos, end token.Pos) {
	var found ast.Stmt
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(ast.Stmt)
		if !ok {
			return true
		}
		if target.Pos() < stmt.Pos() || target.End() > stmt.End() {
			return true
		}
		found = stmt
		return true
	})
	if found != nil {
		return found.Pos(), found.End()
	}
	return target.Pos(), target.End()
}

// mergePairedClosers ensures brackets that opened on the same line also close
// on the same line. For example, if ( and { open on the same line, then } and
// ) must close on the same line. This applies recursively: g.Go(Watch(func() {
// ... })) merges both the Watch `)` and the Go `)` onto the `}` line.
func mergePairedClosers(tokFile *token.File, file *ast.File) {
	sigs := funcSignatures(file)
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if inFuncSignature(sigs, call.Pos()) {
			return true
		}
		var (
			innerClose token.Pos
			lastArg    = call.Args[len(call.Args)-1]
		)
		switch a := lastArg.(type) {
		case *ast.FuncLit:
			innerClose = a.Body.Rbrace
		case *ast.CallExpr:
			innerClose = a.Rparen
		default:
			return true
		}
		if tokFile.Line(call.Lparen) != tokFile.Line(lastArg.Pos()) {
			return true
		}
		if tokFile.Line(innerClose) == tokFile.Line(call.Rparen) {
			return true
		}
		// A // comment between the two closers forces a line break
		// the merge would swallow.
		if lineCommentWithin(file, lastArg.Pos(), call.Rparen) {
			return true
		}
		mergeSpan(tokFile, innerClose, call.Rparen)
		return true
	})
}

// collapseSingleDecls removes the parentheses from a declaration
// block holding one spec: const ( X = 1 ) becomes const X = 1. The
// lines the block occupied are merged with it, so the line table keeps
// counting what go/printer will emit.
func collapseSingleDecls(tokFile *token.File, file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if !ok || !gd.Lparen.IsValid() || len(gd.Specs) != 1 {
			return true
		}
		if gd.Doc != nil || commentWithin(file, gd.Lparen, gd.Rparen) {
			return true
		}
		mergeSpan(tokFile, gd.TokPos, gd.Specs[0].Pos())
		mergeSpan(tokFile, gd.Specs[0].End(), gd.Rparen)
		gd.Lparen, gd.Rparen = token.NoPos, token.NoPos
		return true
	})
}

// spansDeclGroup reports whether a parenthesized declaration group lies
// between two positions. go/printer gives such a group a line per spec
// no matter what the line table says, so merging across one drops lines
// the render keeps and leaves the two out of step.
func spansDeclGroup(file *ast.File, lo, hi token.Pos) (found bool) {
	ast.Inspect(file, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if ok && gd.Lparen.IsValid() && gd.Pos() >= lo && gd.End() <= hi {
			found = true
		}
		return !found
	})
	return found
}

// commentWithin reports whether any comment lies between two
// positions. A pass that rewrites the text between two delimiters
// drops whatever comment sits there, and comments are never deleted.
func commentWithin(file *ast.File, lo, hi token.Pos) bool {
	for _, cg := range file.Comments {
		if cg.Pos() > lo && cg.Pos() < hi {
			return true
		}
	}
	return false
}

// lineCommentWithin reports whether a // comment lies between two
// positions. A // comment runs to end of line, so merging the lines it
// spans would swallow every token that follows it.
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
