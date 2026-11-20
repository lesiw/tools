package unwrap

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"slices"
	"strings"

	"lesiw.io/tools/analyzers/linelen"
)

// printerNormalizeNumbers is the mode bit go/format sets to have go/printer
// rewrite numeric literals to their canonical form. go/printer does not export
// it. The rules start from gofmt's output, so it is named here the way
// go/format names it.
const printerNormalizeNumbers = 1 << 30

var printerCfg = printer.Config{
	Mode:     printer.UseSpaces | printer.TabIndent | printerNormalizeNumbers,
	Tabwidth: 8,
}

// fits reports whether line may stay on one line. It asks about the whole
// line, trailing comment included. A join that fits only because its comment
// was ignored would print a line the width rules reject.
func fits(line []byte) bool { return linelen.CheckLine(line) }

// printNode renders n with the shared printer configuration.
func printNode(fset *token.FileSet, n ast.Node) (string, error) {
	var buf strings.Builder
	if err := printerCfg.Fprint(&buf, fset, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// snapshot is the file as go/printer renders it at one moment, paired with the
// line table that produced that render. The table is copied. A rule that edits
// lines while it iterates then keeps reading the lines it measured.
type snapshot struct {
	lines  [][]byte
	starts []int
	base   int
	end    int
}

// renderer is the file a rule edits, together with the printer's account of
// it. Every width question goes to the printer, because it decides padding as
// well as breaks.
type renderer struct {
	fset    *token.FileSet
	tokFile *token.File
	file    *ast.File
}

// render returns the file as go/printer renders it, split into lines. It
// reports whether the render has one line per line-table entry.
//
// The two can disagree. The printer keeps a multi-line string literal's own
// newlines. It also re-lays-out a list whose items no longer fit on the line
// the table puts them on. A render that disagrees describes a different file
// than the table does, so no width measured against it can be trusted.
func (r *renderer) render() ([][]byte, bool) {
	var buf bytes.Buffer
	if err := printerCfg.Fprint(&buf, r.fset, r.file); err != nil {
		return nil, false
	}
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	// A rendered file ends in a newline, so the split ends in an empty
	// element the table has no line for.
	return lines, len(lines)-1 == r.tokFile.LineCount()
}

// snapshot returns a snapshot of the file. A render that fails yields one with
// no lines. It reports every line as empty and every span as unmeasurable, so
// callers leave the layout alone.
func (r *renderer) snapshot() *snapshot {
	var (
		tokFile = r.tokFile
		snap    = &snapshot{
			starts: slices.Clone(tokFile.Lines()),
			base:   tokFile.Base(),
			end:    tokFile.Base() + tokFile.Size(),
		}
	)
	if lines, ok := r.render(); ok {
		snap.lines = lines
	}
	return snap
}

// line returns the line number pos held when the snapshot was taken. It
// returns 0 for a position before the file's first line.
func (s *snapshot) line(pos token.Pos) int {
	i, exact := slices.BinarySearch(s.starts, int(pos)-s.base)
	if exact {
		return i + 1
	}
	return i
}

// join merges every line between start and end onto start's line. It keeps the
// merge only when the printer agrees with it. The file still renders one line
// per line-table entry, and the line that results is not one the width rules
// reject. A merge that fits on paper can still print too long, because the
// printer pads a declaration group to the longest name in it.
func join(r *renderer, start, end token.Pos) {
	mergeLines(r, start, end, alignment{}, false)
}

// joinAligned merges like join, and measures every line of the alignment the
// span sits in. The padding a merge adds to its columns lengthens the lines
// around it, and the merged span does not cover those.
func joinAligned(r *renderer, start, end token.Pos, a alignment) {
	mergeLines(r, start, end, a, false)
}

// joinExempt merges like join for a span the width rules exempt. Only the
// printer has a say.
func joinExempt(r *renderer, start, end token.Pos) {
	mergeLines(r, start, end, alignment{}, true)
}

// alignment is the span of a construct the printer aligns, with the render it
// had before the merge. The zero alignment is none.
type alignment struct {
	lo, hi   token.Pos
	previous [][]byte
}

func mergeLines(r *renderer, start, end token.Pos, a alignment, exempt bool) {
	if sameLine(r.tokFile, start, end) {
		return
	}
	if a.lo.IsValid() {
		// The alignment's other lines take their padding from the
		// merge, so they are compared with the render from before it.
		// A render that disagrees with the line table compares nothing.
		if lines, ok := r.render(); ok {
			a.previous = lines
		}
	}
	before := slices.Clone(r.tokFile.Lines())
	mergeSpan(r.tokFile, start, end)
	r.keeps(before, start, end, a, exempt)
}

// keeps reports whether the edit made since before leaves a line the width
// rules accept, or a line they exempt. It restores the line table when the
// edit fails. A caller that edited the tree undoes that itself.
func (r *renderer) keeps(before []int, pos, end token.Pos, a alignment, exempt bool) bool {
	var (
		lines, ok = r.render()
		at        = lineAt(r.tokFile, pos)
	)
	// A merge can push what followed it onto a line of its own. Every line
	// the span now covers is measured, not only the one it started on.
	last := lineAt(r.tokFile, end)
	if ok && last <= len(lines) && at >= 1 {
		for line := at; line <= last; line++ {
			text := lines[line-1]
			// A line holding a raw string cannot be measured
			// lexically. A backtick seen out of context reads as
			// the start of a string that runs to the end of the
			// line. Such a line is left alone.
			raw := bytes.ContainsRune(text, '`')
			if exempt || (!raw && fits(text)) {
				continue
			}
			r.tokFile.SetLines(before)
			return false
		}
		if a.lo.IsValid() {
			removed := len(before) - r.tokFile.LineCount()
			if !r.keptAligned(lines, a, removed, at) {
				r.tokFile.SetLines(before)
				return false
			}
		}
		return true
	}
	r.tokFile.SetLines(before)
	return false
}

// keptAligned reports whether the merge left the lines around it acceptable.
// It drops the merge only for a line that ends past the width and wider than
// it was.
func (r *renderer) keptAligned(lines [][]byte, a alignment, removed, at int) bool {
	if a.previous == nil || linelen.Config.Len <= 0 {
		return true
	}
	lo, hi := lineAt(r.tokFile, a.lo), lineAt(r.tokFile, a.hi)
	for line := lo; line <= hi; line++ {
		if line == at {
			// The merged line is measured against the width rules.
			continue
		}
		was := line
		if line > at {
			was += removed
		}
		if was < 1 || was > len(a.previous) {
			continue
		}
		text := lines[line-1]
		width := linelen.Width(a.previous[was-1])
		if !fits(text) && linelen.Width(text) > width {
			return false
		}
	}
	return true
}

// lineAt returns the line pos is written on, ignoring any //line directive. A
// directive renumbers positions for the compiler. The line table is indexed by
// the lines the file is written on, so every rule needs the unadjusted number.
func lineAt(tokFile *token.File, pos token.Pos) int {
	return tokFile.PositionFor(pos, false).Line
}

// merged returns the text the span from lo to hi would have if the printer put
// it on one line. It reports whether that text can be predicted at all.
//
// The estimate is only ever used to reject. A span it measures as too long is
// skipped without a render, and a span it accepts still goes to join, which
// measures the printed line. So the estimate is a lower bound. A span it
// rejects is too long however the printer lays it out, and a span it accepts
// is measured again before anything is decided.
//
// A line in an alignment group is printed at a column the printer chooses. A
// raw string or a block comment keeps newlines of its own. A span that holds
// either is not predictable from the render. Everything else concatenates,
// with one space where each line break was and none where the printer might
// drop one.
func (s *snapshot) merged(lo, hi token.Pos) ([]byte, bool) {
	first, last := s.line(lo), s.line(hi)
	if first < 1 || last < first || last > len(s.lines) {
		return nil, false
	}
	merged := slices.Clone(s.lines[first-1])
	if !predictable(merged) {
		return nil, false
	}
	for _, line := range s.lines[first:last] {
		if !predictable(line) {
			return nil, false
		}
		body := bytes.TrimLeft(line, " \t")
		merged = dropTrailingComma(merged, body)
		if needsSpace(merged, body) {
			merged = append(merged, ' ')
		}
		merged = append(merged, body...)
	}
	return merged, true
}

// dropTrailingComma removes the comma a multi-line list keeps before its
// closer, which the printer drops once the closer joins the list.
func dropTrailingComma(merged, body []byte) []byte {
	if len(body) == 0 || len(merged) == 0 || merged[len(merged)-1] != ',' {
		return merged
	}
	switch body[0] {
	case ')', ']', '}':
		return merged[:len(merged)-1]
	}
	return merged
}

// needsSpace reports whether a line break between head and body prints as a
// space. The printer keeps a break as one space unless a delimiter or an
// operator is at the join. A call or a paren takes none; an operand and an
// operator do.
//
// A brace takes none here. It can open a block or a composite literal, and the
// text does not say which. The two do not agree. Counting a space the printer
// drops would make the estimate longer than the printed line, and a span the
// estimate rejects is a join no pass will ever make. Counting none costs a
// render instead, and join measures the printed line.
func needsSpace(head, body []byte) bool {
	if len(head) == 0 || len(body) == 0 {
		return false
	}
	switch head[len(head)-1] {
	case '(', '[', '{', ';', '.':
		return false
	}
	switch body[0] {
	case ')', ']', '}', ',', ';', '.', ':':
		return false
	}
	return true
}

// predictable reports whether the printer prints line as it is written. Such a
// line has no alignment padding and no newline of its own.
func predictable(line []byte) bool {
	body := bytes.TrimLeft(line, " \t")
	if bytes.ContainsRune(body, '`') || bytes.Contains(body, []byte("/*")) {
		return false
	}
	if bytes.IndexByte(body, '\t') >= 0 {
		return false
	}
	return !bytes.Contains(body, []byte("  "))
}

// sameLine reports whether a and b are written on one line.
func sameLine(tokFile *token.File, a, b token.Pos) bool {
	return lineAt(tokFile, a) == lineAt(tokFile, b)
}

func mergeSpan(tokFile *token.File, start, end token.Pos) {
	startLine := lineAt(tokFile, start)
	for lineAt(tokFile, end) > startLine && startLine < tokFile.LineCount() {
		tokFile.MergeLine(startLine)
	}
}
