package main

import (
	"bytes"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// multiLineLits returns every composite literal that spans more than
// one line.
func multiLineLits(tokFile *token.File, file *ast.File) []*ast.CompositeLit {
	var lits []*ast.CompositeLit
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !lit.Lbrace.IsValid() || !lit.Rbrace.IsValid() {
			return true
		}
		if tokFile.Line(lit.Lbrace) != tokFile.Line(lit.Rbrace) {
			lits = append(lits, lit)
		}
		return true
	})
	return lits
}

// nestedInLit reports whether pos falls inside a literal in lits other
// than the one starting at pos itself.
func nestedInLit(lits []*ast.CompositeLit, pos token.Pos) bool {
	for _, lit := range lits {
		if lit.Pos() == pos {
			continue // skip self
		}
		if pos >= lit.Pos() && pos <= lit.End() {
			return true
		}
	}
	return false
}

// litPlan is one composite literal's final shape, decided against the
// snapshot before any of it is applied: whether its braces cuddle with
// its elements', which of its braces move to a line of their own, and
// which of its elements start one. Deciding every literal first is what
// keeps the cuddle rule and the width rule from reading each other's
// half-applied output.
type litPlan struct {
	lit        *ast.CompositeLit
	elts       []*ast.CompositeLit
	cuddle     bool
	splitOpen  bool
	splitClose bool
	eltSplits  []int
}

// layoutLiterals settles every composite literal's shape in one walk.
// Literals are visited outermost first, so by the time one is examined
// every decision that moves it — an ancestor cuddling, an ancestor
// breaking its elements onto their own lines — has been taken and
// folded into the line it is measured against. The plan is applied at
// the end.
//
// Cuddling pulls the braces of a literal holding untyped literals
// together, so
//
//	return []Edit{
//		{
//			Pos: a,
//		},
//		{
//			Pos: b,
//		},
//	}
//
// becomes
//
//	return []Edit{{
//		Pos: a,
//	}, {
//		Pos: b,
//	}}
//
// The elements keep their own layout; only the braces move onto shared
// lines, which costs every element a level of indent it was not using
// for anything. A run of elements that all fit on one line is the
// collapse rules' business and is left alone.
func layoutLiterals(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) {
	applyLitPlans(tokFile, planLiterals(fset, tokFile, file, snap))
}

// planLiterals decides every literal's shape without applying any of
// it. outdent carries what cuddling lifts off a snapshot line, and
// placed carries the lines a broken-open literal's elements land on;
// both are filled by ancestors before a descendant reads them.
func planLiterals(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) []*litPlan {
	var (
		plans   []*litPlan
		placed  []placement
		outdent = make(map[int]int)
		headers = headerSpans(file)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !lit.Lbrace.IsValid() || !lit.Rbrace.IsValid() {
			return true
		}
		if len(lit.Elts) == 0 {
			return true
		}
		ln := snap.line(lit.Lbrace)
		indent := snap.indent(lit.Lbrace)
		lineW := snap.width(ln) - outdent[ln]
		if p, ok := innermost(placed, lit.Lbrace); ok {
			indent, lineW = p.indent, p.width()
		}
		plan := &litPlan{lit: lit}
		plans = append(plans, plan)
		multi := tokFile.Line(lit.Lbrace) != tokFile.Line(lit.Rbrace)

		// A literal of literals wears its braces cuddled once an
		// element takes more than a line — either because it already
		// does, or because this literal is about to be broken open and
		// the element will.
		elts, uniform := untypedLits(lit.Elts)
		if uniform && seamsClear(file, lit, elts) {
			var takesLines bool
			switch {
			case anyMultiLine(tokFile, elts):
				takesLines = true
			case !multi:
				// This literal is about to be broken open, and its
				// elements go with it.
				takesLines = lineW > cfg.Len
			default:
				takesLines = anyWillSplit(snap, outdent, elts)
			}
			if takesLines {
				plan.elts, plan.cuddle = elts, true
			}
		}
		if plan.cuddle {
			// Everything between the braces comes one level toward the
			// margin, and each seam joins the line a brace closed on
			// with the line the next one opened on. A literal already
			// wearing its braces cuddled has nothing left to move, so
			// it is measured where it already sits.
			var (
				bottom = snap.line(lit.Rbrace)
				inner  = append(slices.Clone(indent), '\t')
			)
			if !cuddledAlready(snap, lit, plan.elts) {
				for l := ln + 1; l < bottom; l++ {
					outdent[l] += cfg.Tab
				}
			}
			for _, s := range seams(lit, plan.elts) {
				placed = append(placed, seamLine(snap, outdent, s))
			}
			// An element that was on one line of its own takes a line
			// of its own between the seams.
			for _, e := range plan.elts {
				if snap.line(e.Lbrace) != snap.line(e.Rbrace) {
					continue
				}
				items, ok := renderAll(fset, e.Elts)
				if !ok {
					continue
				}
				placed = append(placed, placement{
					lo:     e.Lbrace,
					hi:     e.Rbrace,
					indent: inner,
					text: string(inner) +
						strings.Join(items, ", ") + ",",
				})
			}
			return true
		}
		if inHeader(headers, lit.Pos()) {
			// Expanding a literal inside a header would wrap the
			// header; headers stay single-line.
			return true
		}
		if multi {
			planMultiLineSplits(tokFile, snap, outdent, plan)
			return true
		}
		// Single-line: only expand when the line itself is overlong.
		// A single-line literal nested inside a multi-line parent is
		// explicitly allowed — being nested does not force a split.
		if lineW <= cfg.Len {
			return true
		}
		// For single-line expansions the elements move onto lines one
		// level in from this literal, packed the way an argument list
		// is, so no line they land on runs past the limit.
		items, ok := renderAll(fset, lit.Elts)
		if !ok {
			return true
		}
		inner := width(indent) + cfg.Tab
		plan.eltSplits = packLines(items, inner)[1:]
		plan.splitOpen, plan.splitClose = true, true
		placed = append(placed, eltLines(fset, plan, indent)...)
		return true
	})
	return plans
}

// eltLines returns the lines a broken-open literal's elements come to
// occupy, one per run of elements that share a line, so a literal
// nested in one of them is measured where it lands.
func eltLines(fset *token.FileSet, plan *litPlan, indent []byte) (out []placement) {
	var (
		lit    = plan.lit
		inner  = append(slices.Clone(indent), '\t')
		starts = append([]int{0}, plan.eltSplits...)
	)
	for i, start := range starts {
		last := len(lit.Elts) - 1
		if i+1 < len(starts) {
			last = starts[i+1] - 1
		}
		items, ok := renderAll(fset, lit.Elts[start:last+1])
		if !ok {
			return out
		}
		out = append(out, placement{
			lo:     lit.Elts[start].Pos(),
			hi:     lit.Elts[last].End(),
			indent: inner,
			text:   string(inner) + strings.Join(items, ", ") + ",",
		})
	}
	return out
}

// seamLine returns the line a seam's two brace lines become once
// merged, with the indent cuddling takes off them already removed.
func seamLine(snap *snapshot, outdent map[int]int, seam [2]token.Pos) placement {
	var (
		head = outdented(snap, outdent, snap.line(seam[0]))
		tail []byte
		sep  string
	)
	if snap.line(seam[0]) != snap.line(seam[1]) {
		// Braces already sharing a line have nothing to join.
		tail = bytes.TrimLeft(snap.text(snap.line(seam[1])), " \t")
		if joinsWithBlank(head, tail) {
			sep = " "
		}
	}
	return placement{
		lo:     snap.lineStart(seam[0]),
		hi:     snap.lineEnd(seam[1]),
		indent: slices.Clone(extractIndent(head)),
		text:   string(head) + sep + string(tail),
	}
}

// outdented returns a line's text with the indent cuddling takes off it
// removed.
func outdented(snap *snapshot, outdent map[int]int, line int) []byte {
	text := snap.text(line)
	for range outdent[line] / cfg.Tab {
		if len(text) == 0 || text[0] != '\t' {
			break
		}
		text = text[1:]
	}
	return text
}

// splitSeams puts an element back on a line of its own wherever
// cuddling left a seam over the width limit. It runs on the settled
// layout, so the seam it measures is the one that will be printed,
// nested splits and all.
func splitSeams(tokFile *token.File, file *ast.File, snap *snapshot) {
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !lit.Lbrace.IsValid() || len(lit.Elts) == 0 {
			return true
		}
		elts, uniform := untypedLits(lit.Elts)
		if !uniform {
			return true
		}
		for _, s := range seams(lit, elts) {
			if snap.line(s[0]) != snap.line(s[1]) {
				continue
			}
			if snap.width(snap.line(s[0])) <= cfg.Len {
				continue
			}
			addNewline(tokFile, s[1])
		}
		return true
	})
}

// cuddledAlready reports whether every seam already shares a line, so
// cuddling would move nothing.
func cuddledAlready(snap *snapshot, lit *ast.CompositeLit, elts []*ast.CompositeLit) bool {
	for _, s := range seams(lit, elts) {
		if snap.line(s[0]) != snap.line(s[1]) {
			return false
		}
	}
	return true
}

// renderAll renders each expression, reporting false if any will not
// print.
func renderAll(fset *token.FileSet, exprs []ast.Expr) ([]string, bool) {
	out := make([]string, 0, len(exprs))
	for _, e := range exprs {
		s, err := printNode(fset, e)
		if err != nil {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// seamsClear reports whether cuddling would swallow no comment.
func seamsClear(file *ast.File, lit *ast.CompositeLit, elts []*ast.CompositeLit) bool {
	for _, s := range seams(lit, elts) {
		if commentWithin(file, s[0], s[1]) {
			return false
		}
	}
	return true
}

// seams returns the brace pairs cuddling brings together: the literal's
// opener with its first element's, each element's closer with the next
// element's opener, and the last element's closer with the literal's.
func seams(lit *ast.CompositeLit, elts []*ast.CompositeLit) [][2]token.Pos {
	out := [][2]token.Pos{{lit.Lbrace, elts[0].Lbrace}}
	for i, e := range elts[1:] {
		out = append(out, [2]token.Pos{elts[i].Rbrace, e.Lbrace})
	}
	last := elts[len(elts)-1]
	return append(out, [2]token.Pos{last.Rbrace, lit.Rbrace})
}

// applyLitPlans carries out the decisions: the seams close first, then
// the braces and elements that take a line of their own.
func applyLitPlans(tokFile *token.File, plans []*litPlan) {
	for _, plan := range plans {
		if plan.cuddle {
			for _, s := range seams(plan.lit, plan.elts) {
				mergeSpan(tokFile, s[0], s[1])
			}
		}
	}
	for _, plan := range plans {
		var (
			lit   = plan.lit
			first = lit.Elts[0].Pos()
			last  = lit.Elts[len(lit.Elts)-1].Pos()
		)
		for _, i := range plan.eltSplits {
			addNewline(tokFile, lit.Elts[i].Pos())
		}
		if plan.splitOpen && tokFile.Line(lit.Lbrace) == tokFile.Line(first) {
			addNewline(tokFile, first)
		}
		if plan.splitClose && tokFile.Line(last) == tokFile.Line(lit.Rbrace) {
			addNewline(tokFile, lit.Rbrace)
		}
	}
}

// untypedLits returns the elements as composite literals, reporting
// false unless every one is a composite literal that elides its type
// and carries both braces.
func untypedLits(elts []ast.Expr) ([]*ast.CompositeLit, bool) {
	out := make([]*ast.CompositeLit, 0, len(elts))
	for _, e := range elts {
		lit, ok := e.(*ast.CompositeLit)
		if !ok || lit.Type != nil {
			return nil, false
		}
		if !lit.Lbrace.IsValid() || !lit.Rbrace.IsValid() {
			return nil, false
		}
		out = append(out, lit)
	}
	return out, true
}

// anyWillSplit reports whether any single-line literal sits on a line
// the width rules are going to break open, which is what makes it span
// lines by the time the plan is applied.
func anyWillSplit(snap *snapshot, outdent map[int]int, lits []*ast.CompositeLit) bool {
	for _, lit := range lits {
		ln := snap.line(lit.Lbrace)
		if snap.width(ln)-outdent[ln] > cfg.Len {
			return true
		}
	}
	return false
}

// anyMultiLine reports whether any literal spans more than one line.
func anyMultiLine(tokFile *token.File, lits []*ast.CompositeLit) bool {
	for _, lit := range lits {
		if tokFile.Line(lit.Lbrace) != tokFile.Line(lit.Rbrace) {
			return true
		}
	}
	return false
}

// planMultiLineSplits decides the braces and elements of a literal that
// already spans lines.
func planMultiLineSplits(tokFile *token.File, snap *snapshot, outdent map[int]int, plan *litPlan) {
	var (
		lineWidth = func(ln int) int { return snap.width(ln) - outdent[ln] }
		lit       = plan.lit
		last      = lit.Elts[len(lit.Elts)-1]
		cuddled   = tokFile.Line(lit.Lbrace)+1 == tokFile.Line(lit.Rbrace)

		// A literal wrapped tight around its elements — opener on the
		// first element's line, closer on the last one's — reads as
		// one bracketed thing however many lines the elements take.
		tight = tokFile.Line(lit.Lbrace) == tokFile.Line(lit.Elts[0].Pos()) &&
			tokFile.Line(lit.Rbrace) == tokFile.Line(last.End())
	)

	// For struct literals that are multi-line (and neither cuddled nor
	// wrapped tight), braces must be on their own lines.
	if !cuddled && !tight && lit.Type != nil {
		plan.splitOpen = hasFieldValuesOnLine(lit, lit.Lbrace, tokFile)
		plan.splitClose = hasFieldValuesOnLine(lit, lit.Rbrace, tokFile)
	}

	overlong := make(map[int]struct{})
	openLine := tokFile.Line(lit.Lbrace)
	closeLine := tokFile.Line(lit.Rbrace)
	for ln := openLine + 1; ln < closeLine; ln++ {
		if lineWidth(ln) > cfg.Len {
			overlong[ln] = struct{}{}
		}
	}
	if len(overlong) == 0 {
		return
	}
	seen := make(map[int]struct{})
	for i, elt := range lit.Elts {
		var (
			ln      = tokFile.Line(elt.Pos())
			_, long = overlong[ln]
			_, dup  = seen[ln]
		)
		if long && dup {
			plan.eltSplits = append(plan.eltSplits, i)
		}
		seen[ln] = struct{}{}
	}
}

// hasFieldValuesOnLine reports whether the given line (by Lbrace or Rbrace
// position) contains any field values. For the opening brace line, this
// means any element starts on that line. For the closing brace line, this
// means any element ends on that line. Cuddled }, { is allowed (no fields
// between the braces on those lines).
func hasFieldValuesOnLine(lit *ast.CompositeLit, brace token.Pos, tokFile *token.File) bool {
	braceLine := tokFile.Line(brace)
	for _, elt := range lit.Elts {
		if tokFile.Line(elt.Pos()) == braceLine {
			return true
		}
		if tokFile.Line(elt.End()) == braceLine {
			return true
		}
	}
	return false
}
