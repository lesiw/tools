// Package bookends reports bracket pairs that disagree about their layout.
//
// Go's printer indents a closer that begins a line to match the line its
// opener begins. A closer that trails the last item of a list takes that
// item's indent instead, so the opener and its own closer end up at two
// different columns. The fix breaks the line before the closer and leaves the
// printer to place it.
//
// The package also owns two shapes that depend on the same braces. A literal
// whose type is elided wears its opening brace at the end of a line, so every
// row of it starts at one column. And a table, a literal whose elements are
// elided literals, is laid out as one edit: a row that cannot fit forces every
// row across lines, with the braces cuddled.
package bookends

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

// Analyzer reports bookends that disagree about their indent.
var Analyzer = &analysis.Analyzer{
	Name: "bookends",
	Doc:  "reports a closing bracket that is not at the indent of its opening bracket",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		check(pass, file, src)
	}
	return nil, nil
}

func check(pass *analysis.Pass, file *ast.File, src []byte) {
	var (
		l     = newLayout(pass.Fset.File(file.FileStart), src)
		pairs []pair
	)
	ast.Inspect(file, func(n ast.Node) bool {
		if p, ok := delimiters(n); ok {
			pairs = append(pairs, p)
		}
		if p, ok := openerPair(n); ok {
			pairs = append(pairs, p)
		}
		if p, ok := tablePair(n, l, pass); ok {
			pairs = append(pairs, p)
		}
		return true
	})
	// A table is one edit, so the pairs inside it stand down.
	var tables []pair
	for _, p := range pairs {
		if p.table {
			tables = append(tables, p)
		}
	}
	inTable := func(pos token.Pos) bool {
		for _, t := range tables {
			if pos > t.open && pos <= t.close {
				return true
			}
		}
		return false
	}
	// Innermost first: a closer that moves is in place before the pair
	// around it is measured.
	slices.SortFunc(pairs, func(a, b pair) int {
		return cmp.Compare(b.open, a.open)
	})
	for _, p := range pairs {
		if !p.table && inTable(p.open) {
			continue
		}
		if p.table {
			if tableDone(l, p.lit) {
				continue // the table already reads as a table
			}
			// A comment inside the table would not survive a
			// rebuild from the elements, and a comment is the
			// author's. The table is reported and left alone.
			if hasInteriorComment(l, file, p.lit) {
				continue
			}
			// A table is one edit: the braces, the seams and the row
			// breaks overlap, so they are decided together.
			text, ok := tableText(pass, l, src, p.lit)
			if !ok {
				continue
			}
			start := token.Pos(sourceLineStart(src,
				int(p.lit.Lbrace)-l.base,
			) + l.base)
			pass.Report(analysis.Diagnostic{
				Pos:     p.open,
				Message: "table mixes one-line and multi-line rows",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Put every row on its own lines",
					TextEdits: []analysis.TextEdit{{
						Pos:     start,
						End:     p.close + 1,
						NewText: text,
					}},
				}},
			})
			continue
		}
		if p.opener {
			// An elided type that spans lines wears its opening
			// brace at the end of a line, so every row starts at
			// the same column.
			if l.line(p.open) == l.line(p.close) {
				continue // a one-line row keeps its shape
			}
			if l.line(p.open) == l.line(p.first) {
				pass.Report(analysis.Diagnostic{
					Pos:     p.open,
					Message: "opening bracket shares a line with its first element",
					SuggestedFixes: []analysis.SuggestedFix{{
						Message: "Move the first element onto its own line",
						TextEdits: []analysis.TextEdit{{
							Pos: p.open + 1,
							End: p.open + 1,
							NewText: append([]byte("\n"),
								l.indentAt(p.open)...,
							),
						}},
					}},
				})
				l.breakAt(p.open+1, l.indentAt(p.open))
			}
			continue
		}
		if l.beginsLine(p.close) {
			continue // the printer indents it to match the opener
		}
		want := l.indentAt(p.open)
		if slices.Equal(want, l.indentAt(p.close)) {
			continue
		}
		// The text between the last item and the closer is kept: a
		// comment there stays where the author put it, with the comma
		// ahead of it and the break behind it. A spread ends the
		// argument it belongs to, so the comma follows it.
		tail := string(src[int(p.lastEnd)-l.base : int(p.close)-l.base])
		var at int
		if strings.HasPrefix(tail, "...") {
			at = len("...")
		}
		if !strings.Contains(tail[at:], ",") {
			tail = tail[:at] + "," + tail[at:]
		}
		pass.Report(analysis.Diagnostic{
			Pos:     p.close,
			Message: "closer is not at the indent of its opener",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "Move the closer onto its own line",
				TextEdits: []analysis.TextEdit{{
					Pos:     p.lastEnd,
					End:     p.close,
					NewText: append([]byte(tail+"\n"), want...),
				}},
			}},
		})
		l.breakAt(p.close, want)
	}
}

// pair is one bracket pair: where it opens, where it closes, and the end of
// the last item before the closer.
type pair struct {
	open, close token.Pos
	lastEnd     token.Pos
	first       token.Pos // first element, for an opener pair
	lit         *ast.CompositeLit
	opener      bool // the pair guards an opener, not a closer
	table       bool // the pair guards a whole table
}

// sourceLineStart returns the offset of the newline that ends the line before
// off, plus one. It reads the source, so a break this pass simulated does not
// move it.
func sourceLineStart(src []byte, off int) int {
	for i := off - 1; i >= 0; i-- {
		if src[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// tablePair returns the pair for a table: a literal whose elements are all
// literals with an elided type, where at least one row spans lines.
func tablePair(n ast.Node, l *layout, pass *analysis.Pass) (pair, bool) {
	lit, ok := n.(*ast.CompositeLit)
	if !ok || len(lit.Elts) < 2 {
		return pair{}, false
	}
	var multi, wide bool
	for _, elt := range lit.Elts {
		row, ok := elt.(*ast.CompositeLit)
		if !ok || row.Type != nil {
			return pair{}, false
		}
		if l.line(row.Lbrace) != l.line(row.Rbrace) {
			multi = true
			continue
		}
		if rowWide(pass, l, row) {
			wide = true
		}
	}
	if !multi && !wide {
		return pair{}, false
	}
	return pair{open: lit.Lbrace, close: lit.Rbrace, lit: lit, table: true},
		true
}

// rowWide reports whether a row on one line would run past the width limit. A
// table with one such row is a table that has to be read across lines, and
// every row follows it there.
func rowWide(pass *analysis.Pass, l *layout, row *ast.CompositeLit) bool {
	if linelen.Config.Len <= 0 {
		return false // no width was asked for
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, pass.Fset, row); err != nil {
		return false
	}
	line := append(slices.Clone(l.indentAt(row.Lbrace)), buf.Bytes()...)
	return !linelen.CheckLine(line)
}

// tableDone reports whether a table already has every row across lines with
// its braces cuddled: {{ at the start, }, { between rows, }} at the end.
func tableDone(l *layout, lit *ast.CompositeLit) bool {
	var (
		first = lit.Elts[0].(*ast.CompositeLit)
		last  = lit.Elts[len(lit.Elts)-1].(*ast.CompositeLit)
	)
	if l.line(lit.Lbrace) != l.line(first.Lbrace) {
		return false
	}
	if l.line(last.Rbrace) != l.line(lit.Rbrace) {
		return false
	}
	for i, elt := range lit.Elts {
		row := elt.(*ast.CompositeLit)
		if len(row.Elts) == 0 {
			// An empty row cannot be expanded, so it keeps its
			// braces together whatever the rest of the table does.
			continue
		}
		if l.line(row.Lbrace) == l.line(row.Rbrace) {
			return false
		}
		if i == 0 {
			continue
		}
		prev := lit.Elts[i-1].(*ast.CompositeLit)
		if l.line(prev.Rbrace) != l.line(row.Lbrace) {
			return false
		}
	}
	return true
}

// hasInteriorComment reports whether a comment of the author's lies inside the
// table, where rebuilding it from the elements would drop the comment. A
// comment on the opener's line trails the brace and rides with the head.
func hasInteriorComment(l *layout, file *ast.File, lit *ast.CompositeLit) bool {
	open := l.line(lit.Lbrace)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if c.Pos() <= lit.Lbrace || c.End() > lit.Rbrace {
				continue
			}
			if l.line(c.Pos()) != open {
				return true
			}
		}
	}
	return false
}

// tableText renders a table with every row across lines and the braces
// cuddled. It replaces the line the literal starts on, so the text is written
// from a known column rather than from the middle of a line.
func tableText(pass *analysis.Pass, l *layout, src []byte, lit *ast.CompositeLit) ([]byte, bool) {
	var (
		start  = l.lineStart(lit.Lbrace)
		head   = src[start : int(lit.Lbrace)-l.base]
		b      strings.Builder
		indent = l.indentAt(lit.Lbrace)
		inner  = append(slices.Clone(indent), '\t')
	)
	b.Write(head)
	b.WriteString("{")
	for i, elt := range lit.Elts {
		row := elt.(*ast.CompositeLit)
		if len(row.Elts) == 0 {
			// An empty row cannot be expanded, so it keeps its
			// braces on the line they start on.
			b.WriteString("{}")
		} else {
			b.WriteString("{\n")
			for _, e := range row.Elts {
				var buf bytes.Buffer
				if err := format.Node(&buf, pass.Fset, e); err != nil {
					return nil, false
				}
				b.Write(inner)
				b.Write(buf.Bytes())
				b.WriteString(",\n")
			}
			b.Write(indent)
			b.WriteString("}")
		}
		if i < len(lit.Elts)-1 {
			b.WriteString(", ")
			continue
		}
		b.WriteString("}")
	}
	return []byte(b.String()), true
}

// openerPair returns the pair for a literal whose type is elided and which
// spans more than one line. A row of a table wears its opening brace at the
// end of a line, so every row starts at the same column. A literal that names
// its type keeps its first element on the line the type is on.
func openerPair(n ast.Node) (pair, bool) {
	lit, ok := n.(*ast.CompositeLit)
	if !ok || lit.Type != nil || len(lit.Elts) == 0 {
		return pair{}, false
	}
	return pair{open: lit.Lbrace, close: lit.Rbrace, first: lit.Elts[0].Pos(),
		opener: true,
	}, true
}

// delimiters returns the bracket pair in n, if n has one.
//
// Only a pair that takes a trailing comma is returned. A closer that begins a
// line must follow a comma: any other token at the end of a line takes a
// semicolon with it, so an index, a slice, a paren and a type assertion
// cannot break before their closer at all.
func delimiters(n ast.Node) (pair, bool) {
	switch n := n.(type) {
	case *ast.CallExpr:
		if len(n.Args) == 0 {
			return pair{}, false
		}
		return pair{open: n.Lparen, close: n.Rparen,
			lastEnd: n.Args[len(n.Args)-1].End(),
		}, true
	case *ast.CompositeLit:
		if len(n.Elts) == 0 {
			return pair{}, false
		}
		return pair{open: n.Lbrace, close: n.Rbrace,
			lastEnd: n.Elts[len(n.Elts)-1].End(),
		}, true
	case *ast.IndexListExpr:
		if len(n.Indices) == 0 {
			return pair{}, false
		}
		return pair{open: n.Lbrack, close: n.Rbrack,
			lastEnd: n.Indices[len(n.Indices)-1].End(),
		}, true
	}
	return pair{}, false
}

// layout is a copy of the line table that this analysis can break without
// disturbing the shared file set.
type layout struct {
	starts []int // line start offsets, ascending
	indent map[int][]byte
	src    []byte
	base   int
}

func newLayout(tokFile *token.File, src []byte) *layout {
	l := &layout{
		starts: slices.Clone(tokFile.Lines()),
		indent: make(map[int][]byte, tokFile.LineCount()),
		src:    src,
		base:   tokFile.Base(),
	}
	for _, start := range l.starts {
		l.indent[start] = leadingSpace(src, start)
	}
	return l
}

// line returns the number of the line that contains pos, counting from one.
func (l *layout) line(pos token.Pos) int {
	i, exact := slices.BinarySearch(l.starts, int(pos)-l.base)
	if exact {
		return i + 1
	}
	return i
}

// lineStart returns the offset at which the line containing pos starts.
func (l *layout) lineStart(pos token.Pos) int {
	return l.starts[l.line(pos)-1]
}

// beginsLine reports whether only whitespace precedes pos on its line.
func (l *layout) beginsLine(pos token.Pos) bool {
	off := int(pos) - l.base
	return off <= len(l.src) && len(leadingSpace(l.src, l.lineStart(pos))) ==
		off-l.lineStart(pos)
}

// indentAt returns the indent of the line that contains pos.
func (l *layout) indentAt(pos token.Pos) []byte {
	return l.indent[l.lineStart(pos)]
}

// breakAt records a break before pos. The line it starts takes indent.
func (l *layout) breakAt(pos token.Pos, indent []byte) {
	var (
		off      = int(pos) - l.base
		i, exact = slices.BinarySearch(l.starts, off)
	)
	if exact {
		return
	}
	l.starts = slices.Insert(l.starts, i, off)
	l.indent[off] = indent
}

// leadingSpace returns the whitespace at the start of src's line at start.
func leadingSpace(src []byte, start int) []byte {
	end := start
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return src[start:end]
}
