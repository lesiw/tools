package unwrap

import (
	"cmp"
	"go/ast"
	"go/token"
	"slices"
)

// delimited is one bracketed list: a call's arguments, a function type's
// parameters or results, or a composite literal's elements.
type delimited struct {
	open, close token.Pos

	// lit reports a composite literal's braces rather than a call's
	// parentheses.
	lit bool
}

// collectDelimited returns every bracketed list whose items are on one line,
// in source order. A list whose items already span lines is the author's
// layout and is left alone.
func collectDelimited(fset *token.FileSet, tokFile *token.File, file *ast.File) (result []delimited) {
	var (
		sigs    = signatures(file)
		headers = headerSpans(file)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if len(n.Args) == 0 {
				break
			}
			if inHeader(headers, n.Pos()) {
				// A header is single-line-or-nothing; joining
				// across one would wrap it.
				break
			}
			// Rewriting the span between the delimiters would drop
			// every comment inside it, and comments are never
			// deleted.
			if commentWithin(file, n.Lparen, n.Rparen) {
				break
			}
			if spansLines(tokFile, n.Args) {
				break
			}
			result = append(result, delimited{open: n.Lparen, close: n.Rparen})
		case *ast.FuncType:
			// Declared signatures keep their author-chosen layout,
			// as does anything nested inside one — e.g. a
			// function-typed parameter. FuncLits collapse
			// normally.
			if inSignature(sigs, n.Pos()) {
				break
			}
			for _, l := range []*ast.FieldList{n.Params, n.Results} {
				if l == nil || !l.Opening.IsValid() || len(l.List) == 0 {
					continue
				}
				result = append(result, delimited{
					open: l.Opening, close: l.Closing,
				})
			}
		case *ast.CompositeLit:
			if !n.Lbrace.IsValid() || !n.Rbrace.IsValid() {
				break
			}
			if len(n.Elts) <= 1 || commentWithin(file, n.Lbrace, n.Rbrace) {
				break
			}
			// A literal whose elements sit on lines of their own,
			// or that holds an element spanning lines, was
			// expanded for readability, and joining it would undo
			// that.
			if !allOnOneLine(tokFile, n.Elts) || spansLines(tokFile, n.Elts) {
				break
			}
			result = append(result, delimited{
				open: n.Lbrace, close: n.Rbrace, lit: true,
			})
		}
		return true
	})
	// Source order. A chain hangs its later calls off its earlier ones, so
	// the tree visits the last call first while the line reads left to
	// right.
	slices.SortFunc(result, func(a, b delimited) int {
		return cmp.Compare(a.open, b.open)
	})
	return result
}

// allOnOneLine reports whether every expression starts on the line the first
// one starts on.
func allOnOneLine(tokFile *token.File, exprs []ast.Expr) bool {
	for _, e := range exprs[1:] {
		if !sameLine(tokFile, exprs[0].Pos(), e.Pos()) {
			return false
		}
	}
	return true
}

// spansLines reports whether any expression is written across more than one
// line.
func spansLines(tokFile *token.File, exprs []ast.Expr) bool {
	for _, e := range exprs {
		if !sameLine(tokFile, e.Pos(), e.End()) {
			return true
		}
	}
	return false
}

// collapseDelimited pulls multi-line delimited expressions whose merged line
// the printer and the width rules both permit back onto one line.
func collapseDelimited(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
		lits    = multiLineLits(tokFile, file)
		snap    = r.snapshot()
	)
	for _, d := range collectDelimited(r.fset, tokFile, file) {
		if sameLine(tokFile, d.open, d.close) {
			continue
		}
		// A composite literal nested inside a multi-line composite
		// literal was intentionally expanded for readability.
		if d.lit && nestedInLit(lits, d.open) {
			continue
		}
		// A list whose merged text is already too long stays too long.
		// No width rule accepts it. See collapseWrappedLines.
		if merged, ok := snap.merged(d.open, d.close); ok && !fits(merged) {
			continue
		}
		join(r, d.open, d.close)
	}
}
