package main

import (
	"bytes"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"lesiw.io/linelen"
	"lesiw.io/tools/cmd/fmt/internal/printer"
)

// cfg is the line-length authority, shared with the linelen command
// so both tools agree on what counts as an overlong line. The zero
// value carries the defaults.
var cfg linelen.Config

var printerCfg = printer.Config{
	Mode:     printer.UseSpaces | printer.TabIndent,
	Tabwidth: 8,
}

// width returns the visual width of line's code. A trailing line
// comment does not count: a line that only a comment makes overlong
// is not the formatter's to reshape, and linelen still reports it.
func width(line []byte) int {
	return cfg.Width(uncommented(line))
}

// uncommented returns line up to its trailing line comment, with
// trailing whitespace removed. String, rune, and raw string literals
// are respected: a // inside one is content, not a comment.
func uncommented(line []byte) []byte {
	var quote byte
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case quote == '"' || quote == '\'':
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case quote == '`':
			if c == '`' {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			return bytes.TrimRight(line[:i], " \t")
		}
	}
	return line
}

// printNode renders n with the shared printer configuration.
func printNode(fset *token.FileSet, n ast.Node) (string, error) {
	var buf strings.Builder
	if err := printerCfg.Fprint(&buf, fset, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// snapshot is the file as go/printer renders it at one moment, paired
// with the line table that produced that render. A pass measures
// against a snapshot rather than against source bytes: the render
// already carries every line boundary its predecessors added or
// removed, so no pass needs to print, reparse, and start over to see
// the layout it is deciding against.
//
// The line table is copied, not shared. A pass that edits the live
// table while it iterates keeps reading the lines it measured, the way
// a pass reading a byte buffer it is not writing to would.
type snapshot struct {
	lines  [][]byte
	starts []int
	base   int
	end    int
}

// takeSnapshot renders file. A render that fails yields a snapshot with
// no lines, which reports every line as empty and every span as
// unmeasurable, so callers leave the layout alone.
func takeSnapshot(fset *token.FileSet, tokFile *token.File, file *ast.File) *snapshot {
	snap := &snapshot{
		starts: slices.Clone(tokFile.Lines()),
		base:   tokFile.Base(),
		end:    tokFile.Base() + tokFile.Size(),
	}
	var buf bytes.Buffer
	if err := printerCfg.Fprint(&buf, fset, file); err != nil {
		return snap
	}
	snap.lines = bytes.Split(buf.Bytes(), []byte("\n"))
	return snap
}

// line returns the 1-based line pos held when the snapshot was taken.
func (s *snapshot) line(pos token.Pos) int {
	i, exact := slices.BinarySearch(s.starts, int(pos)-s.base)
	if exact {
		return i + 1
	}
	return i
}

// lineStart returns the first position on pos's line, as the line ran
// when the snapshot was taken.
func (s *snapshot) lineStart(pos token.Pos) token.Pos {
	if i := s.line(pos); i > 0 {
		return token.Pos(s.starts[i-1] + s.base)
	}
	return token.Pos(s.base)
}

// lineEnd returns the last position on pos's line, as the line ran when
// the snapshot was taken.
func (s *snapshot) lineEnd(pos token.Pos) token.Pos {
	if i := s.line(pos); i > 0 && i < len(s.starts) {
		return token.Pos(s.starts[i] + s.base - 1)
	}
	return token.Pos(s.end)
}

// text returns the rendered text of a line, or nil when the line is out
// of range. An out-of-range line reads as empty, so a width test on it
// never reports an overlong line the render cannot confirm.
func (s *snapshot) text(line int) []byte {
	if line < 1 || line > len(s.lines) {
		return nil
	}
	return s.lines[line-1]
}

// width returns the visual width of a rendered line.
func (s *snapshot) width(line int) int { return width(s.text(line)) }

// indent returns a copy of the whitespace beginning the rendered line
// that holds pos.
func (s *snapshot) indent(pos token.Pos) []byte {
	return slices.Clone(extractIndent(s.text(s.line(pos))))
}

// extractIndent returns the leading spaces and tabs of line.
func extractIndent(line []byte) []byte {
	for i, b := range line {
		if b != '\t' && b != ' ' {
			return line[:i]
		}
	}
	return line
}

// mergedWidth predicts the visual width of the line that results from
// merging every line the span [start, end] touches. Lines after the
// first contribute their content with leading whitespace stripped, plus
// the blank go/printer writes at each seam — the shape MergeLine plus
// go/printer produces. It reports false for a span the snapshot cannot
// measure, which callers read as "leave this alone".
func (s *snapshot) mergedWidth(start, end token.Pos) (int, bool) {
	if !start.IsValid() || !end.IsValid() || start > end {
		return 0, false
	}
	lo, hi := s.line(start), s.line(end)
	if lo < 1 || hi > len(s.lines) {
		return 0, false
	}
	var (
		total int
		prev  []byte
	)
	for ln := lo; ln <= hi; ln++ {
		seg := s.text(ln)
		if ln > lo {
			seg = bytes.TrimLeft(seg, " \t")
			if joinsWithBlank(prev, seg) {
				total++
			}
		}
		total += width(seg)
		prev = seg
	}
	return total, true
}

// joinsWithBlank reports whether go/printer separates the tail of prev
// from the head of next with a blank once the two lines are one. An
// opening delimiter or selector dot binds tight to what follows, and a
// closing delimiter, comma, or selector dot binds tight to what
// precedes; every other token pair is spaced. Ambiguous seams — braces,
// which abut in composite literals but not around block bodies — count
// the blank, keeping the estimate on the conservative side.
func joinsWithBlank(prev, next []byte) bool {
	prev = bytes.TrimRight(prev, " \t")
	if len(prev) == 0 || len(next) == 0 {
		return false
	}
	switch prev[len(prev)-1] {
	case '(', '[', '.':
		return false
	}
	switch next[0] {
	case ')', ']', ',', '.':
		return false
	}
	return true
}

// mergeSpan merges every line between start and end onto start's line.
func mergeSpan(tokFile *token.File, start, end token.Pos) {
	startLine := tokFile.Line(start)
	for tokFile.Line(end) > startLine && startLine < tokFile.LineCount() {
		tokFile.MergeLine(startLine)
	}
}

// addNewline inserts a line boundary at the offset of at, unless one is
// already there.
func addNewline(tokFile *token.File, at token.Pos) {
	var (
		offset    = tokFile.Offset(at)
		lines     = tokFile.Lines()
		i, exists = slices.BinarySearch(lines, offset)
	)
	if exists {
		return
	}
	tokFile.SetLines(slices.Insert(lines, i, offset))
}
