package fmt

import (
	"bytes"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// layoutLiterals cuddles the braces of a literal that holds literals. It does
// this once an element takes more than a line. Cuddling costs every element a
// level of indent it was not using for anything. The cuddle is kept only when
// every line it produces fits.
//
// Literals are visited outermost first. By the time one is examined, every
// ancestor decision that moves it has been taken and folded into the line it
// is measured against. A run of elements that all fit on one line belongs to
// the collapse rules and is left alone.
func layoutLiterals(r *renderer) {
	applyLitPlans(r, planLiterals(r))
}

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

// litPlan is one composite literal's cuddle. It is decided against the
// snapshot before any of it is applied.
type litPlan struct {
	lit     *ast.CompositeLit
	elts    []*ast.CompositeLit
	cuddle  bool
	cuddled bool
}

// planLiterals decides every cuddle without applying any of it. outdent
// carries how many indent levels cuddling lifts off a snapshot line. placed
// carries the lines the cuddle produces. Ancestors fill both before a
// descendant reads them.
func planLiterals(r *renderer) []*litPlan {
	var (
		fset    = r.fset
		tokFile = r.tokFile
		file    = r.file
		snap    = r.snapshot()

		plans   []*litPlan
		placed  []placement
		outdent = make(map[int]int)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !lit.Lbrace.IsValid() || !lit.Rbrace.IsValid() {
			return true
		}
		if len(lit.Elts) == 0 {
			return true
		}
		indent := snap.indent(lit.Lbrace)
		if p, ok := innermost(placed, lit.Lbrace); ok {
			indent = p.indent()
		}
		plan := &litPlan{lit: lit}
		plans = append(plans, plan)
		// A literal of literals wears its braces cuddled once an
		// element takes more than a line.
		elts, uniform := untypedLits(lit.Elts)
		if !uniform || !seamsClear(file, lit, elts) {
			return true
		}
		if !anyMultiLine(tokFile, elts) {
			return true
		}
		// Everything between the braces moves one level toward the
		// margin. Each seam joins the line a brace closed on with the
		// line the next one opened on. A literal already wearing its
		// braces cuddled has nothing left to move, so it is measured
		// where it already sits.
		ln, bottom := snap.line(lit.Lbrace), snap.line(lit.Rbrace)
		inner := append(slices.Clone(indent), '\t')
		cuddled := cuddledAlready(snap, lit, elts)
		moved := make(map[int]int)
		if !cuddled {
			for l := ln + 1; l < bottom; l++ {
				outdent[l]++
				moved[l] = outdent[l]
			}
		}
		var lines []placement
		for _, s := range seams(lit, elts) {
			lines = append(lines, seamLine(snap, outdent, s))
		}
		// An element that was on one line of its own takes a line of
		// its own between the seams.
		for _, e := range elts {
			if snap.line(e.Lbrace) != snap.line(e.Rbrace) {
				continue
			}
			items, ok := renderAll(fset, e.Elts)
			if !ok {
				continue
			}
			lines = append(lines, placement{
				lo: e.Lbrace,
				hi: e.Rbrace,
				text: string(inner) +
					strings.Join(items, ", ") + ",",
			})
		}
		// The cuddle is kept only when every line it produces fits.
		// That question is about this literal alone, so the decision
		// does not depend on what another rule did.
		for _, l := range lines {
			if !fits([]byte(l.text)) {
				for at, n := range moved {
					if n == 1 {
						delete(outdent, at)
						continue
					}
					outdent[at] = n - 1
				}
				return true
			}
		}
		plan.elts, plan.cuddle, plan.cuddled = elts, true, cuddled
		placed = append(placed, lines...)
		return true
	})
	return plans
}

// seamLine returns the line a seam's two brace lines become once merged. The
// indent cuddling takes off them is already removed.
func seamLine(snap *snapshot, outdent map[int]int, seam [2]token.Pos) placement {
	var (
		head = outdented(snap, outdent, snap.line(seam[0]))
		tail []byte
		sep  string
	)
	if !snap.onOneLine(seam[0], seam[1]) {
		// Braces already sharing a line have nothing to join. The
		// estimate counts the space the printer drops when two braces
		// abut. It errs one column wide rather than one short.
		tail = bytes.TrimLeft(snap.text(snap.line(seam[1])), " \t")
		if len(tail) > 0 {
			sep = " "
		}
	}
	return placement{
		lo:   snap.lineStart(seam[0]),
		hi:   snap.lineEnd(seam[1]),
		text: string(head) + sep + string(tail),
	}
}

// outdented returns a line's text without the indent cuddling takes off it.
// Each outdent unit removes one level.
func outdented(snap *snapshot, outdent map[int]int, line int) []byte {
	text := snap.text(line)
	for range outdent[line] {
		if len(text) == 0 || text[0] != '\t' {
			break
		}
		text = text[1:]
	}
	return text
}

// cuddledAlready reports whether every seam of the literal already shares a
// line. Cuddling it would move nothing.
func cuddledAlready(snap *snapshot, lit *ast.CompositeLit, elts []*ast.CompositeLit) bool {
	for _, s := range seams(lit, elts) {
		if snap.line(s[0]) != snap.line(s[1]) {
			return false
		}
	}
	return true
}

// renderAll renders each expression. It reports false when the printer fails
// on any of them.
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

// seams returns the brace pairs cuddling brings together. Each pair is the
// literal's opener with its first element's, one element's closer with the
// next element's opener, or the last element's closer with the literal's.
func seams(lit *ast.CompositeLit, elts []*ast.CompositeLit) [][2]token.Pos {
	out := [][2]token.Pos{{lit.Lbrace, elts[0].Lbrace}}
	for i, e := range elts[1:] {
		out = append(out, [2]token.Pos{elts[i].Rbrace, e.Lbrace})
	}
	last := elts[len(elts)-1]
	return append(out, [2]token.Pos{last.Rbrace, lit.Rbrace})
}

// applyLitPlans closes each seam a cuddle pulls together. A seam is joined
// only when the line it produces fits. Cuddling never makes a line too long.
func applyLitPlans(r *renderer, plans []*litPlan) {
	for _, plan := range plans {
		if !plan.cuddle || plan.cuddled {
			continue
		}
		for _, s := range seams(plan.lit, plan.elts) {
			join(r, s[0], s[1])
		}
	}
}

// untypedLits returns the elements as composite literals. It reports false
// unless every one of them is a composite literal with no type and both
// braces.
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

// anyMultiLine reports whether any literal spans more than one line.
func anyMultiLine(tokFile *token.File, lits []*ast.CompositeLit) bool {
	for _, lit := range lits {
		if !sameLine(tokFile, lit.Lbrace, lit.Rbrace) {
			return true
		}
	}
	return false
}
