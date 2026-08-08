package main

import (
	"bytes"
	"cmp"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// delimited is one bracketed list — a call's arguments, a function
// type's parameters or results, or a composite literal's elements —
// reduced to the rendered text of its items and the positions the
// layout passes act on. Every pass below shapes a delimited by adding
// or removing line boundaries at those positions, so nested lists edit
// independently and no edit invalidates another's positions.
type delimited struct {
	open, close token.Pos
	items       []string
	itemPos     []token.Pos
	itemEnd     []token.Pos
	lastEnd     token.Pos

	// funcType marks the params or results of an [ast.FuncType].
	// Signatures stay on one line regardless of length, so expand
	// never wraps these; collapse may still pull them together.
	funcType bool

	// lit marks a composite literal's braces.
	lit bool
}

func collectDelimited(fset *token.FileSet, tokFile *token.File, file *ast.File) (result []delimited) {
	var (
		sigs    = funcSignatures(file)
		headers = headerSpans(file)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if len(n.Args) == 0 {
				break
			}
			if inHeader(headers, n.Pos()) {
				// A header is single-line-or-nothing; packing
				// must never wrap inside one.
				break
			}
			// Reflowing the span between the delimiters would drop
			// every comment inside it, and comments are never
			// deleted.
			if commentWithin(file, n.Lparen, n.Rparen) {
				break
			}
			list, pos, end, ok := renderItems(fset, n.Args)
			if !ok {
				return true
			}
			last := end[len(end)-1]
			// Variadic spread (f(xs...)) lives on the CallExpr, not
			// on its last arg, so re-emit it onto the last item
			// before any reflow rebuilds the argument list.
			if n.Ellipsis.IsValid() {
				list[len(list)-1] += "..."
				last = n.Ellipsis + token.Pos(len("..."))
			}
			result = append(result, delimited{
				open: n.Lparen, close: n.Rparen,
				items: list, itemPos: pos, itemEnd: end,
				lastEnd: last,
			})
		case *ast.FuncType:
			// Declared signatures (funcs, methods, interface
			// methods, function-type declarations) keep their
			// author-chosen layout, as does anything nested inside
			// one — e.g. a function-typed parameter. FuncLits
			// collapse normally.
			if inFuncSignature(sigs, n.Pos()) {
				break
			}
			for _, l := range []*ast.FieldList{n.Params, n.Results} {
				if l == nil || !l.Opening.IsValid() || len(l.List) == 0 {
					continue
				}
				if commentWithin(file, l.Opening, l.Closing) {
					continue
				}
				list, pos, end, ok := fields(fset, l.List)
				if !ok {
					continue
				}
				result = append(result, delimited{
					open:     l.Opening,
					close:    l.Closing,
					items:    list,
					itemPos:  pos,
					itemEnd:  end,
					lastEnd:  end[len(end)-1],
					funcType: true,
				})
			}
		case *ast.CompositeLit:
			if !n.Lbrace.IsValid() || !n.Rbrace.IsValid() {
				break
			}
			if len(n.Elts) <= 1 || !allOnSameLine(tokFile, n.Elts) {
				break
			}
			if commentWithin(file, n.Lbrace, n.Rbrace) {
				break
			}
			list, pos, end, ok := renderItems(fset, n.Elts)
			if !ok {
				return true
			}
			result = append(result, delimited{
				open: n.Lbrace, close: n.Rbrace,
				items: list, itemPos: pos, itemEnd: end,
				lastEnd: end[len(end)-1],
				lit:     true,
			})
		}
		return true
	})
	// Source order, not tree order: a chain hangs its later calls off
	// its earlier ones, so the tree visits the last call first, while
	// the line is laid out left to right. Deciding in reading order is
	// what lets a call that has already been broken open tell the ones
	// after it which line they came to rest on.
	slices.SortFunc(result, func(a, b delimited) int {
		return cmp.Compare(a.open, b.open)
	})
	return result
}

// renderItems renders each expression on one line with its span,
// reporting ok=false when any renders multi-line.
func renderItems(fset *token.FileSet, exprs []ast.Expr) (out []string, pos, end []token.Pos, ok bool) {
	out = make([]string, 0, len(exprs))
	pos = make([]token.Pos, 0, len(exprs))
	end = make([]token.Pos, 0, len(exprs))
	for _, e := range exprs {
		s, err := printNode(fset, e)
		if err != nil || strings.Contains(s, "\n") {
			return nil, nil, nil, false
		}
		out = append(out, s)
		pos = append(pos, e.Pos())
		end = append(end, e.End())
	}
	return out, pos, end, true
}

// fields renders a field list into one item per field.
func fields(fset *token.FileSet, list []*ast.Field) (out []string, pos, end []token.Pos, ok bool) {
	out = make([]string, 0, len(list))
	pos = make([]token.Pos, 0, len(list))
	end = make([]token.Pos, 0, len(list))
	for _, f := range list {
		s, err := printField(fset, f)
		if err != nil {
			return nil, nil, nil, false
		}
		out = append(out, s)
		pos = append(pos, f.Pos())
		end = append(end, f.End())
	}
	return out, pos, end, true
}

func printField(fset *token.FileSet, f *ast.Field) (string, error) {
	typ, err := printNode(fset, f.Type)
	if err != nil {
		return "", err
	}
	if len(f.Names) == 0 {
		return typ, nil
	}
	names := make([]string, 0, len(f.Names))
	for _, name := range f.Names {
		names = append(names, name.Name)
	}
	return strings.Join(names, ", ") + " " + typ, nil
}

// collapsedWidth predicts the width of the line d occupies once its
// span is on one line. Merging joins the rendered lines with the blanks
// go/printer writes at each seam; the comma trailing the last item of a
// broken list is the one byte the merge drops.
func (d delimited) collapsedWidth(snap *snapshot) (int, bool) {
	w, ok := snap.mergedWidth(d.open, d.close)
	if !ok {
		return 0, false
	}
	if snap.line(d.lastEnd) != snap.line(d.close) {
		w--
	}
	return w, true
}

// delimiters returns the pair of bytes d opens and closes with.
func (d delimited) delimiters() (open, close byte) {
	if d.lit {
		return '{', '}'
	}
	return '(', ')'
}

// tail returns what follows d on the line it currently occupies, which
// is the text that comes to rest on d's closing line once d is broken
// open. It reports false when d's rendered extent cannot be located
// unambiguously on that line, leaving the text after the closer to be
// measured where the snapshot found it.
func (d delimited) tail(line string) (string, bool) {
	open, close := d.delimiters()
	extent := string(open) + strings.Join(d.items, ", ") + string(close)
	i := strings.Index(line, extent)
	if i < 0 || strings.Contains(line[i+1:], extent) {
		return "", false
	}
	return line[i+len(extent):], true
}

// itemsOnOneLine reports whether every item of d starts on one line.
func (d delimited) itemsOnOneLine(snap *snapshot) bool {
	for _, pos := range d.itemPos[1:] {
		if snap.line(pos) != snap.line(d.itemPos[0]) {
			return false
		}
	}
	return true
}

// collapseDelimited pulls multi-line delimited expressions that fit
// back onto one line.
func collapseDelimited(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) {
	lits := multiLineLits(tokFile, file)
	for _, d := range collectDelimited(fset, tokFile, file) {
		if tokFile.Line(d.open) == tokFile.Line(d.close) {
			continue
		}
		w, ok := d.collapsedWidth(snap)
		if !ok || w > cfg.Len {
			continue
		}
		// A composite literal nested inside a multi-line composite
		// literal was intentionally expanded for readability.
		if d.lit && nestedInLit(lits, d.open) {
			continue
		}
		mergeSpan(tokFile, d.open, d.close)
	}
}

// placement is a line expandDelimited laid out: the span of source it
// carries, the indent it starts at, and the text it will render as. A
// list nested inside a placement is measured against the line it was
// just moved onto rather than the line the snapshot found it on, which
// is what keeps the outer and inner lists of one expression from both
// wrapping when only the outer one had to.
type placement struct {
	lo, hi token.Pos
	indent []byte
	text   string
}

func (p placement) width() int { return width([]byte(p.text)) }

// expandDelimited bin-packs delimited expressions that exceed the width
// limit. Items always start on a new line after the opener (no
// half-wrapped cuddling), and the closer takes a line of its own.
func expandDelimited(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) {
	var placed []placement
	for _, d := range collectDelimited(fset, tokFile, file) {
		// Function signatures stay on one line regardless of length.
		if d.funcType {
			continue
		}
		var (
			indent  []byte
			line    string
			region  = snap.lineEnd(d.close)
			oneLine = snap.line(d.open) == snap.line(d.close)
		)
		if p, ok := innermost(placed, d.open); ok {
			if p.width() <= cfg.Len {
				continue
			}
			indent, line, region = p.indent, p.text, p.hi
		} else {
			if oneLine {
				w, ok := d.collapsedWidth(snap)
				if !ok || w <= cfg.Len {
					continue
				}
			} else if !d.itemsOnOneLine(snap) {
				// Already multi-line with items on separate lines.
				continue
			}
			indent = snap.indent(d.open)
			line = string(snap.text(snap.line(d.open)))
		}
		// Whatever trailed the closer comes to rest on the closer's
		// line, at the opener's indent.
		if tail, ok := d.tail(line); ok && oneLine {
			_, close := d.delimiters()
			placed = append(placed, placement{
				lo:     d.close,
				hi:     region,
				indent: slices.Clone(indent),
				text:   string(indent) + string(close) + tail,
			})
		}
		inner := append(slices.Clone(indent), '\t')
		var (
			starts = packLines(d.items, width(inner))
			prev   = d.open
		)
		for i, start := range starts {
			breakBefore(tokFile, snap, prev, d.itemPos[start])
			prev = d.itemPos[start]
			last := len(d.items) - 1
			if i+1 < len(starts) {
				last = starts[i+1] - 1
			}
			placed = append(placed, placement{
				lo:     d.itemPos[start],
				hi:     d.itemEnd[last],
				indent: inner,
				text: string(inner) +
					strings.Join(d.items[start:last+1], ", ") + ",",
			})
		}
		breakBefore(tokFile, snap, d.lastEnd, d.close)
	}
}

// innermost returns the narrowest placement holding pos.
func innermost(placed []placement, pos token.Pos) (placement, bool) {
	var (
		out   placement
		found bool
	)
	for _, p := range placed {
		if pos < p.lo || pos > p.hi {
			continue
		}
		if !found || p.lo > out.lo {
			out, found = p, true
		}
	}
	return out, found
}

// breakBefore starts a new line at pos when pos still shares a line
// with prev. A position that already opens its line gets no boundary:
// inserting one past the indent would leave the indent behind as a
// blank line of its own.
func breakBefore(tokFile *token.File, snap *snapshot, prev, pos token.Pos) {
	if snap.line(prev) == snap.line(pos) {
		addNewline(tokFile, pos)
	}
}

// alignClosers moves a closer that sits at a different indent than its
// opener onto a line of its own, where go/printer indents it to match.
func alignClosers(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) {
	for _, d := range collectDelimited(fset, tokFile, file) {
		if snap.line(d.open) == snap.line(d.close) {
			continue
		}
		if bytes.Equal(snap.indent(d.open), snap.indent(d.close)) {
			continue
		}
		breakBefore(tokFile, snap, d.lastEnd, d.close)
	}
}

// packLines bin-packs items onto lines indented by indentWidth columns,
// each line ending in a comma. It returns the index of the item that
// opens each line.
func packLines(items []string, indentWidth int) []int {
	if len(items) == 0 {
		return nil
	}
	var (
		starts    = []int{0}
		lineWidth = indentWidth
	)
	// Every packed line ends in a comma, so it counts against the
	// limit the same as any other column.
	limit := cfg.Len - len(",")
	for i, item := range items {
		itemWidth := width([]byte(item))
		if i > starts[len(starts)-1] {
			if lineWidth+len(", ")+itemWidth > limit {
				starts = append(starts, i)
				lineWidth = indentWidth
			} else {
				lineWidth += len(", ")
			}
		}
		lineWidth += itemWidth
	}
	return unwidow(starts, len(items))
}

// unwidow moves the last item of the penultimate line onto the final
// line when that line holds a lone item, so no item is stranded. It
// rebalances by item, never by text: an item may itself contain the
// separator inside a string literal.
func unwidow(starts []int, total int) []int {
	n := len(starts)
	if n < 2 || total-starts[n-1] != 1 || starts[n-1]-starts[n-2] < 2 {
		return starts
	}
	starts[n-1]--
	return starts
}

func allOnSameLine(tokFile *token.File, elts []ast.Expr) bool {
	if len(elts) == 0 {
		return true
	}
	line := tokFile.Line(elts[0].Pos())
	for _, e := range elts[1:] {
		if tokFile.Line(e.Pos()) != line {
			return false
		}
	}
	return true
}
