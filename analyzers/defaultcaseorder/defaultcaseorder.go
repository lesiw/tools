// Package defaultcaseorder provides an analysis.Analyzer that reports a
// default clause in the middle of a switch, a type switch, or a select.
//
// A default clause belongs at the top of the clause list or at the end of it.
// The clause that runs is the same either way, so the position is
// presentational: this is a house rule, and the suite writes the default
// clause where a reader looks for it.
//
// A default clause that a case falls through into is not reported: moving it
// changes where the jump lands. One that ends in a fallthrough is not reported
// either: at the end of the list it would have no clause to enter.
//
// The fix moves the clause last. A three-way comparison written as >, =, <
// loses that order, and //ignore is how a switch keeps it. The README lists
// the cases that report without a fix.
//
// The rule is adapted from go-critic's defaultCaseOrder, which reports the
// same shape in an expression switch and offers no fix.
package defaultcaseorder

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "defaultcaseorder",
	Doc:  "reports a default clause in the middle of a switch, a type switch, or a select",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tf := pass.Fset.File(file.FileStart)
		src, err := pass.ReadFile(tf.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", tf.Name(), err)
		}
		s := &source{File: tf, src: src, comments: file.Comments}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SwitchStmt:
				s.check(pass, n.Body)
			case *ast.TypeSwitchStmt:
				s.check(pass, n.Body)
			case *ast.SelectStmt:
				s.check(pass, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// source carries the source text a fix is spliced into.
type source struct {
	*token.File

	src      []byte
	comments []*ast.CommentGroup

	// spans holds the text this run has offered to rewrite, so that one fix
	// per run is offered and the order they apply in is fixed.
	spans []span
}

type span struct {
	lo, hi int
}

// clause is one clause of a switch or select body, reduced to what the rule
// needs to know about it.
type clause struct {
	pos       token.Pos
	isDefault bool

	// fallsThrough reports whether the clause ends by passing control to the
	// clause below it, which may carry a label.
	fallsThrough bool
}

func clausesOf(body *ast.BlockStmt) []clause {
	out := make([]clause, 0, len(body.List))
	for _, stmt := range body.List {
		switch stmt := stmt.(type) {
		case *ast.CaseClause:
			out = append(out, clause{
				pos:          stmt.Pos(),
				isDefault:    stmt.List == nil,
				fallsThrough: endsInFallthrough(stmt.Body),
			})
		case *ast.CommClause:
			out = append(out, clause{
				pos:          stmt.Pos(),
				isDefault:    stmt.Comm == nil,
				fallsThrough: endsInFallthrough(stmt.Body),
			})
		}
	}
	return out
}

// check reports each default clause that sits between two cases.
func (s *source) check(pass *analysis.Pass, body *ast.BlockStmt) {
	list := clausesOf(body)
	if len(list) == 0 {
		return
	}
	last := list[len(list)-1]
	for i := 1; i < len(list)-1; i++ {
		c := list[i]
		if !c.isDefault {
			continue
		}
		if list[i-1].fallsThrough || c.fallsThrough {
			continue
		}
		diag := analysis.Diagnostic{
			Pos:     c.pos,
			Message: "default clause is in the middle",
		}
		if edits := s.move(body, c, list[i+1], last); edits != nil {
			diag.SuggestedFixes = []analysis.SuggestedFix{{
				Message:   "Move the default clause last",
				TextEdits: edits,
			}}
		}
		pass.Report(diag)
	}
}

// move returns the edits that delete c from its place and write it after the
// last clause. It returns nil when one of the guards below holds.
func (s *source) move(body *ast.BlockStmt, c, next, last clause) []analysis.TextEdit {
	// The file may have changed since it was parsed, and then every offset
	// below points into other text.
	if s.Size() != len(s.src) {
		return nil
	}
	// The move deletes whole lines, so the clause has to own its first one.
	if !s.leadsLine(c.pos) {
		return nil
	}
	// It writes the clause at the start of the brace's line, which the brace
	// has to own.
	if !s.leadsLine(body.Rbrace) {
		return nil
	}
	from, stranded := s.clauseStart(c.pos)
	if stranded {
		return nil
	}
	// The clause ends where the next one begins, and the next one has to
	// begin on a line of its own.
	if !s.leadsLine(next.pos) {
		return nil
	}
	to, _ := s.clauseStart(next.pos)
	if to <= from {
		return nil
	}
	if !s.clearBrace(body, s.indentAt(last.pos)) {
		return nil
	}
	at := s.lineOffset(body.Rbrace)
	if s.claimed(from, to) {
		return nil
	}
	s.spans = append(s.spans, span{from, to})
	return []analysis.TextEdit{{
		Pos: s.Pos(from),
		End: s.Pos(to),
	}, {
		Pos:     s.Pos(at),
		NewText: s.src[from:to],
	}}
}

// claimed reports whether a fix this run has offered rewrites text that
// overlaps [lo, hi).
func (s *source) claimed(lo, hi int) bool {
	for _, t := range s.spans {
		if lo < t.hi && t.lo < hi {
			return true
		}
	}
	return false
}

// clauseStart returns the offset of the first line the clause carries: the
// comment groups written above it at its own indentation, and the blank lines
// above those, which it keeps as a separator. The clause below it keeps its
// own blank lines.
//
// It also reports whether a comment group the clause cannot carry sits above
// it. That group stays, and once the clause has moved the group reads as a
// comment on the clause below it.
func (s *source) clauseStart(pos token.Pos) (start int, stranded bool) {
	var (
		line   = s.writtenLine(pos)
		indent = s.indentAt(pos)
	)
	// The groups are visited from the clause upward, so each one met makes
	// the next one reachable.
	for _, group := range slices.Backward(s.comments) {
		if group.End() >= pos || s.writtenLine(group.End()) >= line {
			continue
		}
		// Only blank lines may stand between the group and the clause.
		if !s.blank(s.Offset(group.End()), s.Offset(s.LineStart(line))) {
			continue
		}
		if !s.leadsLine(group.Pos()) {
			continue // it trails code on its line, which owns it
		}
		switch own := s.indentAt(group.Pos()); {
		case own == indent:
			line = s.writtenLine(group.Pos())
		case strings.HasPrefix(own, indent):
			// It is written inside the clause above, and it stays there.
			return s.Offset(s.LineStart(line)), false
		default:
			return s.Offset(s.LineStart(line)), true
		}
	}
	for line > 1 && s.blankLine(line-1) {
		line--
	}
	return s.Offset(s.LineStart(line)), false
}

// clearBrace reports whether a clause can be written just above the closing
// brace. A comment group there belongs to no clause below it, so the moved
// clause would take it over; a group indented with the last clause's body
// stays in that body.
func (s *source) clearBrace(body *ast.BlockStmt, indent string) bool {
	line := s.writtenLine(body.Rbrace)
	for _, group := range slices.Backward(s.comments) {
		if group.End() >= body.Rbrace || s.writtenLine(group.End()) >= line {
			continue
		}
		// Only blank lines may stand between the group and the brace.
		if !s.blank(s.Offset(group.End()), s.Offset(s.LineStart(line))) {
			continue
		}
		if !s.leadsLine(group.Pos()) {
			continue // it trails code on its line, which owns it
		}
		own := s.indentAt(group.Pos())
		if own != indent && strings.HasPrefix(own, indent) {
			// It is written inside the last clause's body. Look above it.
			line = s.writtenLine(group.Pos())
			continue
		}
		// It belongs to no clause below it, and the move would take it over.
		return false
	}
	return true
}

// lineOffset returns the offset of the first character on the line containing
// pos.
func (s *source) lineOffset(pos token.Pos) int {
	return s.Offset(s.LineStart(s.writtenLine(pos)))
}

// writtenLine returns the line pos is written on. The offsets the fix splices
// are unadjusted, so the lines must be too; the embedded File's Line applies a
// //line directive, which can name a line past the end of the file.
func (s *source) writtenLine(pos token.Pos) int {
	return s.PositionFor(pos, false).Line
}

// leadsLine reports whether only indentation comes before pos on its line.
func (s *source) leadsLine(pos token.Pos) bool {
	return s.blank(s.lineOffset(pos), s.Offset(pos))
}

// indentAt returns the whitespace that leads the line containing pos.
func (s *source) indentAt(pos token.Pos) string {
	var (
		lo = s.lineOffset(pos)
		hi = lo
	)
	for hi < len(s.src) && (s.src[hi] == ' ' || s.src[hi] == '\t') {
		hi++
	}
	return string(s.src[lo:hi])
}

// blank reports whether src[lo:hi] is whitespace.
func (s *source) blank(lo, hi int) bool {
	for _, b := range s.src[lo:hi] {
		switch b {
		case ' ', '\t', '\r', '\n':
		default:
			return false
		}
	}
	return true
}

// blankLine reports whether line holds nothing but whitespace.
func (s *source) blankLine(n int) bool {
	var (
		lo = s.Offset(s.LineStart(n))
		hi = len(s.src)
	)
	if n < s.LineCount() {
		hi = s.Offset(s.LineStart(n + 1))
	}
	return s.blank(lo, hi)
}

// endsInFallthrough reports whether the clause body ends in a fallthrough,
// which may carry a label of its own.
func endsInFallthrough(body []ast.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	last := body[len(body)-1]
	if label, ok := last.(*ast.LabeledStmt); ok {
		last = label.Stmt
	}
	branch, ok := last.(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}
