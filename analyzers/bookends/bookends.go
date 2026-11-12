// Package bookends reports bracket pairs that disagree about their layout.
//
// Go's printer indents a closer that begins a line to match the line its
// opener begins. A closer that trails the last item of a list takes that
// item's indent instead, so the opener and its own closer end up at two
// different columns. The fix breaks the line before the closer and leaves the
// printer to place it.
//
// The package also reports four shapes that depend on the same braces. A
// literal whose type is elided ends its first line with the opening brace, so
// every row starts at one column. A table, a literal whose elements are elided
// literals, is rewritten as one edit: when a row does not fit, every row goes
// on a line of its own, and each row's braces go on the lines their elements
// are on. A list that holds a single indented line is packed: indenting a list
// across two lines to carry one line of elements wastes the indentation. So is
// a list with a line past the width, which cannot stand as written. The layout
// is the one with the least raggedness, the sum of the squared space left at
// the end of each line, so elements spread by width: a short element joins a
// long one.
//
// Two indented lines within the width are a group the author drew. Nothing but
// the layout says so, and a rule that rewrote the group would erase the only
// place the grouping is written down, so the list is left alone.
//
// The list keeps one element per line when its elements are keyed, are
// composite literals, or are a literal's constants, and when a comment sits
// among them. A slog call is the exception to the count: its trailing
// arguments are key-value pairs, so each key keeps the value it labels on one
// line. A keyed literal on one line past the width takes one key per line,
// which is the shape it takes once it no longer fits.
package bookends

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/header"
)

// Analyzer reports bracket layout that does not match the list's shape.
var Analyzer = &analysis.Analyzer{
	Name: "bookends",
	Doc:  "reports bracket layout that does not match the shape of the list",
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

// markInner marks each pair that an outer pair lays out. Packing the inner
// pair first would leave the outer line over the width with no rule left to
// fix it, so the outermost pair is decided first and a pair with no layout to
// write lays out nothing.
func markInner(pass *analysis.Pass, l *layout, file *ast.File, pairs []pair) {
	for i := range slices.Backward(pairs) {
		if pairs[i].inner {
			continue
		}
		if _, _, ok := packPlan(pass, l, file, pairs[i]); !ok {
			continue
		}
		for j := range pairs {
			if contains(pairs[i], pairs[j]) {
				pairs[j].inner = true
			}
		}
	}
}

// contains reports whether inner sits inside outer.
func contains(outer, inner pair) bool {
	return outer.open < inner.open && inner.close < outer.close
}

func check(pass *analysis.Pass, file *ast.File, src []byte) {
	var (
		l       = newLayout(pass.Fset.File(file.FileStart), src)
		pairs   = collectPairs(pass, file, l)
		tables  = tablePairs(pairs)
		headers = header.Spans(file)
	)
	// Innermost first: a closer that moves is in place before the pair
	// around it is measured.
	slices.SortFunc(pairs, func(a, b pair) int {
		return cmp.Compare(b.open, a.open)
	})
	markInner(pass, l, file, pairs)
	for _, p := range pairs {
		if header.In(headers, p.open) {
			continue // a header takes the lines it already has
		}
		if !p.table && inTable(tables, p.open) {
			continue
		}
		switch {
		case p.table:
			reportTable(pass, l, src, file, p)
		case p.opener:
			reportOpener(pass, l, p)
		case overWideKeyed(l, file, p):
			reportExpand(pass, l, p)
		default:
			// Pack measures the source, so it runs before the closer
			// rewrites the break it would measure. A pack moves the
			// closer itself, so the closer rule only runs when the
			// pack leaves the list alone.
			if !reportPack(pass, l, file, p) {
				reportCloser(pass, l, src, p)
			}
		}
	}
}

// collectPairs returns every bracket pair the file holds.
func collectPairs(pass *analysis.Pass, file *ast.File, l *layout) []pair {
	var pairs []pair
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
	return pairs
}

// tablePairs returns the pairs that guard a whole table.
func tablePairs(pairs []pair) (tables []pair) {
	for _, p := range pairs {
		if p.table {
			tables = append(tables, p)
		}
	}
	return tables
}

// inTable reports whether pos sits inside a table. A delimiters pair for the
// table literal opens on the same brace as the table, so the bound is
// inclusive.
func inTable(tables []pair, pos token.Pos) bool {
	for _, t := range tables {
		if pos >= t.open && pos <= t.close {
			return true
		}
	}
	return false
}

// reportTable reports a table that does not read as a table. A comment inside
// it would not survive a rebuild from the elements, so the table is reported
// and left alone.
func reportTable(pass *analysis.Pass, l *layout, src []byte, file *ast.File, p pair) {
	if tableDone(l, p.lit) {
		return // the table already reads as a table
	}
	if hasComment(l, file, p) {
		return
	}
	// A table is one edit: the braces, the seams and the row breaks
	// overlap, so they are decided together.
	text, ok := tableText(pass, l, src, p.lit, file)
	if !ok {
		return
	}
	start := token.Pos(sourceLineStart(src, int(p.lit.Lbrace)-l.base) + l.base)
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
}

// reportOpener reports an elided type that spans lines and ends its first line
// with the opening brace, so every row starts at the same column.
func reportOpener(pass *analysis.Pass, l *layout, p pair) {
	if l.line(p.open) == l.line(p.close) {
		return // a one-line row keeps its shape
	}
	if l.line(p.open) != l.line(p.first) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     p.open,
		Message: "opening bracket shares a line with its first element",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Move the first element onto its own line",
			TextEdits: []analysis.TextEdit{{
				Pos:     p.open + 1,
				End:     p.open + 1,
				NewText: append([]byte("\n"), l.indentAt(p.open)...),
			}},
		}},
	})
	l.breakAt(p.open+1, l.indentAt(p.open))
}

// reportCloser reports a closer that does not sit at the indent of its opener.
func reportCloser(pass *analysis.Pass, l *layout, src []byte, p pair) {
	if l.beginsLine(p.close) {
		return // the printer indents it to match the opener
	}
	want := l.indentAt(p.open)
	if slices.Equal(want, l.indentAt(p.close)) {
		return
	}
	// The text between the last item and the closer is kept: a comment
	// there stays where the author put it, with the comma ahead of it and
	// the break behind it. A spread ends the argument it belongs to, so
	// the comma follows it.
	tail := string(src[int(p.lastEnd)-l.base : int(p.close)-l.base])
	var at int
	if p.ellipsis.IsValid() {
		at = int(p.ellipsis) - int(p.lastEnd) + len("...")
	}
	if !commaBeforeComment(tail[at:]) {
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

// commaBeforeComment reports whether a comma comes before the first comment in
// tail. A comma inside a comment is the comment's text, not the list's.
func commaBeforeComment(tail string) bool {
	for i := 0; i < len(tail); i++ {
		if tail[i] == ',' {
			return true
		}
		if tail[i] == '/' {
			return false
		}
	}
	return false
}

// pair is one bracket pair: where it opens, where it closes, and the end of
// the last item before the closer.
type pair struct {
	open, close token.Pos
	lastEnd     token.Pos
	first       token.Pos // first element, for an opener pair
	elts        []ast.Expr
	lit         *ast.CompositeLit
	opener      bool      // the pair guards an opener, not a closer
	table       bool      // the pair guards a whole table
	spread      bool      // a call whose last argument is a spread
	ellipsis    token.Pos // the ellipsis of a spread
	call        *ast.CallExpr
	inner       bool // the pair around this one lays the line out
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

// trailingComments returns the comments that sit on the table's opening line
// ahead of the first row. The rebuild replaces that line, so they are written
// back explicitly.
func trailingComments(l *layout, file *ast.File, lit *ast.CompositeLit) (out []*ast.Comment) {
	open := l.line(lit.Lbrace)
	for _, c := range commentsIn(file, lit.Lbrace, lit.Rbrace) {
		if l.line(c.Pos()) == open {
			out = append(out, c)
		}
	}
	return out
}

// tableText renders a table with every row across lines and the braces
// cuddled. It replaces the line the literal starts on, so the text is written
// from a known column rather than from the middle of a line.
func tableText(pass *analysis.Pass, l *layout, src []byte, lit *ast.CompositeLit, file *ast.File) ([]byte, bool) {
	var (
		start    = l.lineStart(lit.Lbrace)
		head     = src[start : int(lit.Lbrace)-l.base]
		b        strings.Builder
		indent   = l.indentAt(lit.Lbrace)
		inner    = append(slices.Clone(indent), '\t')
		comments = trailingComments(l, file, lit)
		written  bool
	)
	// A comment ahead of the first row is written back after that row's
	// brace, which is where the author put it.
	writeComments := func() {
		if written {
			return
		}
		written = true
		for _, c := range comments {
			b.WriteString(" ")
			b.Write(src[int(c.Pos())-l.base : int(c.End())-l.base])
		}
	}
	b.Write(head)
	b.WriteString("{")
	for i, elt := range lit.Elts {
		row := elt.(*ast.CompositeLit)
		if len(row.Elts) == 0 {
			// An empty row cannot be expanded, so it keeps its
			// braces on the line they start on.
			b.WriteString("{}")
			writeComments()
		} else {
			b.WriteString("{")
			writeComments()
			b.WriteString("\n")
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
			lastEnd:  n.Args[len(n.Args)-1].End(),
			elts:     n.Args,
			spread:   n.Ellipsis.IsValid(),
			ellipsis: n.Ellipsis,
			call:     n,
		}, true
	case *ast.CompositeLit:
		if len(n.Elts) == 0 {
			return pair{}, false
		}
		return pair{open: n.Lbrace, close: n.Rbrace,
			lastEnd: n.Elts[len(n.Elts)-1].End(),
			elts:    n.Elts,
			lit:     n,
		}, true
	case *ast.IndexListExpr:
		if len(n.Indices) == 0 {
			return pair{}, false
		}
		return pair{open: n.Lbrack, close: n.Rbrack,
			lastEnd: n.Indices[len(n.Indices)-1].End(),
			elts:    n.Indices,
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

// lineText returns the source of the line that contains pos, without its
// newline.
func (l *layout) lineText(pos token.Pos) []byte {
	var (
		start = l.lineStart(pos)
		i     = l.line(pos)
	)
	if i < len(l.starts) {
		return bytes.TrimSuffix(l.src[start:l.starts[i]], []byte("\n"))
	}
	return l.src[start:]
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

// overWideKeyed reports whether the literal holds keys and sits on one line
// that runs past the width, which is the shape that takes one key per line.
func overWideKeyed(l *layout, file *ast.File, p pair) bool {
	if p.lit == nil || !keyed(p.elts) {
		return false
	}
	if l.line(p.open) != l.line(p.close) {
		return false // the literal already holds more than one line
	}
	if len(commentsIn(file, p.open, p.close)) > 0 {
		return false // a comment would be moved by the rewrite
	}
	return !linelen.CheckLine(l.lineText(p.open))
}

// reportExpand reports a keyed literal that runs past the width and suggests
// one key per line, which is the shape a keyed literal takes once it no longer
// fits.
func reportExpand(pass *analysis.Pass, l *layout, p pair) {
	var (
		indent = l.indentAt(p.open)
		inner  = append(slices.Clone(indent), '\t')
		out    []byte
	)
	out = append(out, "{\n"...)
	for _, e := range p.elts {
		text, ok := render(pass, e)
		if !ok {
			return
		}
		out = append(out, inner...)
		out = append(out, text...)
		out = append(out, ",\n"...)
	}
	out = append(out, indent...)
	out = append(out, '}')
	pass.Report(analysis.Diagnostic{
		Pos:     p.open,
		End:     p.close + 1,
		Message: "keyed literal runs past the width",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Put one key per line",
			TextEdits: []analysis.TextEdit{{
				Pos:     p.open,
				End:     p.close + 1,
				NewText: out,
			}},
		}},
	})
}

// reportPack reports a list whose elements are not laid out as the width
// allows.
func reportPack(pass *analysis.Pass, l *layout, file *ast.File, p pair) bool {
	f, indented, ok := packPlan(pass, l, file, p)
	if !ok {
		return false
	}
	var msg, fixMsg string
	switch {
	case f.pulled:
		msg = "element fits on the opener's line"
		fixMsg = "Pull the element up"
	case f.keyed:
		msg = "each key and its value take a line"
		fixMsg = "Split the key-value arguments"
	default:
		msg = fmt.Sprintf("elements fit on %d lines", f.lines)
		if cur := indented; cur > f.lines {
			msg += fmt.Sprintf(", not %d", cur)
		}
		fixMsg = "Pack the elements"
	}
	// The edit starts just after the opener, so a comment on that line is
	// replaced by it and is written back first.
	text := openerComments(l, file, p)
	text = append(text, f.text...)
	pos := p.open
	if f.pulled {
		pos = f.pos
	}
	pass.Report(analysis.Diagnostic{
		Pos:     pos,
		Message: msg,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fixMsg,
			TextEdits: []analysis.TextEdit{{
				Pos:     p.open + 1,
				End:     p.close,
				NewText: text,
			}},
		}},
	})
	return true
}

// packPlan returns the layout the pack rule would write for the list, the
// number of lines the list holds now, and whether the rule would write one. A
// list the pair around it lays out is not this rule's to pack.
func packPlan(pass *analysis.Pass, l *layout, file *ast.File, p pair) (layoutFill, int, bool) {
	if p.inner || !packable(pass, p) {
		return layoutFill{}, 0, false
	}
	// A list whose one element line holds a single element written across
	// lines moves that element up to the opener's line, which holds one line
	// at an indent level either way and takes fewer lines.
	if up, ok := pullUp(l, file, p); ok && l.elementLines(p) == 1 {
		return up, 1, true
	}
	if len(p.elts) < 2 {
		return layoutFill{}, 0, false
	}
	// A comment inside the list stops the join rule, so a list that would
	// fit on one line without the comment is still this rule's to lay out.
	// The join rule also refuses a list broken into two indented lines, so
	// such a list is this rule's when the elements are pairs.
	joinable := l.indented(p) < 2 &&
		len(commentsIn(file, p.open, p.close)) == 0
	if joinable && joinFits(pass, l, p) {
		return layoutFill{}, 0, false // the join rule's work
	}
	if hasComment(l, file, p) {
		return layoutFill{}, 0, false // a packed line would move it
	}
	f, ok := fill(pass, l, file, p)
	if !ok {
		return layoutFill{}, 0, false // no layout fits
	}
	indented := l.indented(p)
	if !wanted(l, file, p, f, indented) {
		return layoutFill{}, 0, false
	}
	return f, indented, true
}

// pullUp returns the layout that moves a lone element up to the opener's line,
// for a list whose one element line holds a single element written across
// lines. Both layouts hold one line at an indent level, so the rule prefers
// the one with fewer lines.
func pullUp(l *layout, file *ast.File, p pair) (layoutFill, bool) {
	if len(p.elts) == 0 {
		return layoutFill{}, false
	}
	var (
		open = l.line(p.open)
		line = l.line(p.elts[0].Pos())
	)
	if open == line {
		return layoutFill{}, false // the element is on the opener's line
	}
	if !l.beginsLine(p.close) {
		return layoutFill{}, false // the closer shares the element's last line
	}
	// The element line holds one element, and that element spans lines.
	var on []ast.Expr
	for _, e := range p.elts {
		if l.line(e.Pos()) == line {
			on = append(on, e)
		}
	}
	if len(on) != 1 || l.line(on[0].Pos()) == l.line(on[0].End()) {
		return layoutFill{}, false
	}
	// A comment either side of the element is not part of its text, so the
	// edit would drop it.
	ahead := commentsIn(file, p.open, on[0].Pos())
	behind := commentsIn(file, on[0].End(), p.close)
	if len(ahead) > 0 || len(behind) > 0 {
		return layoutFill{}, false
	}
	text := l.src[int(on[0].Pos())-l.base : int(on[0].End())-l.base]
	if p.spread {
		text = append(slices.Clone(text), "..."...)
	}
	head := l.src[l.lineStart(p.open) : int(p.open)-l.base+1]
	if !linelen.CheckLine(append(slices.Clone(head), firstLine(text)...)) {
		return layoutFill{}, false // the pulled line would run past the width
	}
	return layoutFill{
		text:   slices.Clone(text),
		lines:  l.indented(p),
		pulled: true,
		pos:    on[0].Pos(),
	}, true
}

// elementLines returns the number of lines below the opener's line on which an
// element starts.
func (l *layout) elementLines(p pair) int {
	var (
		open = l.line(p.open)
		seen = make(map[int]struct{}, len(p.elts))
	)
	for _, e := range p.elts {
		if line := l.line(e.Pos()); line != open {
			seen[line] = struct{}{}
		}
	}
	return len(seen)
}

// indented returns the number of lines the list holds under its opener's line.
// A line an element spans counts, so a list whose argument is a func literal
// holds every line the literal holds.
func (l *layout) indented(p pair) int {
	var (
		open = l.line(p.open)
		cl   = l.line(p.close)
		n    = cl - open - 1
	)
	if !l.beginsLine(p.close) {
		n++ // the closer shares the last element's line, which counts
	}
	return max(n, 0)
}

// wanted reports whether the list's layout is not the one the rule asks for.
// One indented line is the width's wrap. Two or more are a group the author
// drew, and the rule leaves the group alone. A key-value list is the
// exception, since the pairs are the structure, and so is a line the width
// does not allow, since the group cannot stand as written.
func wanted(l *layout, file *ast.File, p pair, f layoutFill, indented int) bool {
	if indented != 1 && !f.keyed && !overWide(l, file, p) {
		return false
	}
	// An element that spans lines and sits alone on its line is the fault
	// again, one level down: the line holds nothing but an opener. A list
	// with no line to share has no layout to offer.
	if f.alone && !f.shared && !f.keyed {
		return false
	}
	// The fill starts every line at the element indent, so a list whose
	// opener shares its line with elements is never already laid out.
	if l.line(p.open) == l.line(p.elts[0].Pos()) {
		return true
	}
	return !slices.Equal(f.breaks, breaks(l, p))
}

// packable reports whether the list is one this rule may pack: the elements
// are not keyed, are not names, and are not a literal's rows or constants.
func packable(pass *analysis.Pass, p pair) bool {
	// A literal's elements are its rows or its column, and either is read
	// down; a call's arguments are neither, so both guards apply only to a
	// literal.
	if p.lit != nil && (constant(pass, p.elts) || rows(p.elts)) {
		return false
	}
	return !keyed(p.elts) && !names(p.elts)
}

// names reports whether every element is a name. A list of names is a catalog,
// and the column of names is the order a reader scans it in.
func names(elts []ast.Expr) bool {
	for _, e := range elts {
		switch e := e.(type) {
		case *ast.Ident:
		case *ast.SelectorExpr:
			if _, ok := e.X.(*ast.Ident); !ok {
				return false
			}
		case *ast.CallExpr:
			if len(e.Args) != 0 || !names([]ast.Expr{e.Fun}) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// constant reports whether every element is a constant. A table of names,
// coefficients or digits is read down a column, so it keeps one value per
// line.
func constant(pass *analysis.Pass, elts []ast.Expr) bool {
	for _, e := range elts {
		if pass.TypesInfo.Types[e].Value == nil {
			return false
		}
	}
	return true
}

// rows reports whether every element is a composite literal, which makes the
// list a table. A table's rows stay one per line, whether or not a row names
// its type.
func rows(elts []ast.Expr) bool {
	for _, e := range elts {
		if _, ok := e.(*ast.CompositeLit); !ok {
			return false
		}
	}
	return true
}

// keyed reports whether any element names itself, as in a struct literal.
func keyed(elts []ast.Expr) bool {
	for _, e := range elts {
		if _, ok := e.(*ast.KeyValueExpr); ok {
			return true
		}
	}
	return false
}

// joinFits reports whether the whole list would fit on one line, which is the
// join rule's work rather than this one's.
func joinFits(pass *analysis.Pass, l *layout, p pair) bool {
	if linelen.Config.Len <= 0 {
		return true // without a width there is nothing to pack against
	}
	var (
		start = l.lineStart(p.open)
		head  = l.src[start : int(p.open)-l.base+1]
		line  = slices.Clone(head)
	)
	for i, e := range p.elts {
		if i > 0 {
			line = append(line, ", "...)
		}
		text, ok := render(pass, e)
		if !ok {
			return false
		}
		line = append(line, text...)
	}
	if p.spread {
		line = append(line, "..."...)
	}
	line = append(line, l.src[int(p.close)-l.base])
	return linelen.CheckLine(line)
}

// hasComment reports whether a comment sits among the elements. A comment
// between two of them is inside the edit that rewrites the list, and one ahead
// of the first explains that element alone. A comment on the opener's line
// stays on that line, as it does in a table, unless it sits inside an element:
// an element is written from its own text, which does not carry a comment.
func hasComment(l *layout, file *ast.File, p pair) bool {
	open := l.line(p.open)
	for _, c := range commentsIn(file, p.open, p.close) {
		if l.line(c.Pos()) != open {
			return true
		}
		for _, e := range p.elts {
			if c.Pos() >= e.Pos() && c.End() <= e.End() {
				return true
			}
		}
	}
	return false
}

// openerComments returns the text of the comments on the opener's line, each
// preceded by the space the source had before it, so an edit that replaces the
// line can write them back.
func openerComments(l *layout, file *ast.File, p pair) (out []byte) {
	line := l.line(p.open)
	for _, c := range commentsIn(file, p.open, p.elts[0].Pos()) {
		if l.line(c.Pos()) != line {
			continue
		}
		out = append(out, ' ')
		out = append(out, l.src[int(c.Pos())-l.base:int(c.End())-l.base]...)
	}
	return out
}

// commentsIn returns the comments between lo and hi. A comment that ends
// exactly at hi is inside, since a block comment can abut the bracket.
func commentsIn(file *ast.File, lo, hi token.Pos) (out []*ast.Comment) {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if c.Pos() > lo && c.End() <= hi {
				out = append(out, c)
			}
		}
	}
	return out
}

// layoutFill is one layout of a list: the text of the interior, where each
// line of it begins, and how many lines it uses.
type layoutFill struct {
	text   []byte
	breaks []int // breaks[i] is the index of the first element on line i+1
	lines  int
	keyed  bool      // the elements are key-value pairs, one per line
	shared bool      // some line holds two elements or more
	alone  bool      // some line holds one element that spans lines
	pulled bool      // the element moved up to the opener's line
	pos    token.Pos // where the diagnostic anchors
}

// overWide reports whether a line of the list's own layout is longer than the
// width allows. Lines inside an element are the element's business, so only
// the lines an element starts on are measured, and a line linelen exempts is
// not measured at all: a declared signature wraps as its author chose, and the
// join rule restores that wrap whatever its width.
func overWide(l *layout, file *ast.File, p pair) bool {
	seen := make(map[int]struct{}, len(p.elts))
	for _, e := range p.elts {
		line := l.line(e.Pos())
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		if inSignature(file, e.Pos()) {
			continue
		}
		if !linelen.CheckLine(l.lineText(e.Pos())) {
			return true
		}
	}
	return false
}

// inSignature reports whether pos falls in a span linelen exempts as a
// signature: a function or method declaration up to its body, or a method in
// an interface. A signature holds a list only in an array length, as in
// func f(x [len([]int{1, 2, 3})]int), and measuring that line would leave the
// join rule restoring the wrap forever.
func inSignature(file *ast.File, pos token.Pos) bool {
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Type == nil {
				return true
			}
			end := n.Type.End()
			if n.Body != nil {
				end = n.Body.Lbrace
			}
			found = pos >= n.Pos() && pos <= end
		case *ast.InterfaceType:
			for _, field := range n.Methods.List {
				if _, ok := field.Type.(*ast.FuncType); !ok {
					continue
				}
				if pos >= field.Pos() && pos <= field.End() {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// fill returns the layout of the element list with the least raggedness: the
// sum of the squared space left at the end of each line, which is the line
// breaker TeX uses. Elements spread across lines by width, so a short element
// joins a long one. A layout holds two lines or more, since a list that fits
// on one line is the join rule's work, and every line must fit the width; when
// none does, fill reports no layout.
//
// Elements are grouped first. A key-value argument list groups each key with
// its value, and the groups take a line each, so a key never parts from the
// value it labels.
func fill(pass *analysis.Pass, l *layout, file *ast.File, p pair) (layoutFill, bool) {
	indent := l.indentAt(p.elts[0].Pos())
	if l.line(p.open) == l.line(p.elts[0].Pos()) {
		indent = append(slices.Clone(l.indentAt(p.open)), '\t')
	}
	pk := &packer{
		indent:      indent,
		closeIndent: l.indentAt(p.open),
		texts:       make([][]byte, len(p.elts)),
		starts:      make([]int, len(p.elts)),
		solo:        make([]bool, len(p.elts)),
		spans:       make([]bool, len(p.elts)),
		trail:       make([][]byte, len(p.elts)),
		spread:      p.spread,
		limit:       linelen.Config.Len,
	}
	for i := range pk.starts {
		pk.starts[i] = i // every element is a group of its own
	}
	for i, e := range p.elts {
		if l.line(e.Pos()) != l.line(e.End()) {
			// An element that spans lines keeps the text the author
			// wrote, comments included, and takes a line of its own.
			pk.solo[i] = true
			pk.spans[i] = true
			pk.texts[i] = l.src[int(e.Pos())-l.base : int(e.End())-l.base]
			continue
		}
		text, ok := render(pass, e)
		if !ok {
			return layoutFill{}, false
		}
		pk.texts[i] = text
	}
	var keyed bool
	if starts, ok := keyedArgs(pass, p.call); ok {
		pk.starts, pk.oneGroup, keyed = starts, true, true
	}
	if !pk.trailComments(l, file, p, keyed) {
		return layoutFill{}, false
	}
	next, ok := pk.cheapest()
	if !ok {
		return layoutFill{}, false
	}
	f := pk.layout(next)
	f.keyed = keyed
	return f, true
}

// trailComments marks every element that a comment on the opener's line
// follows, and keeps the comment's text, so the element ends the line the
// comment ends. It reports whether the list can hold the comments at all: a
// key that trails a comment cannot keep the value it labels on the line, so
// such a list has no layout.
func (pk *packer) trailComments(l *layout, file *ast.File, p pair, keyed bool) bool {
	for _, c := range commentsIn(file, p.elts[0].Pos(), p.close) {
		if l.line(c.Pos()) != l.line(p.open) {
			continue
		}
		// The comment trails the last element that ends before it.
		trailed := -1
		for i, e := range p.elts {
			if c.Pos() >= e.End() {
				trailed = i
			}
		}
		if trailed < 0 {
			continue
		}
		// A comment between a key and its value would part the two,
		// but one after the last value leaves the pair whole.
		last := trailed+1 == len(p.elts)
		if keyed && !last && !slices.Contains(pk.starts, trailed+1) {
			return false
		}
		pk.solo[trailed] = true
		text := l.src[int(c.Pos())-l.base : int(c.End())-l.base]
		if pk.trail[trailed] != nil {
			pk.trail[trailed] = append(pk.trail[trailed], ' ')
		}
		pk.trail[trailed] = append(pk.trail[trailed], text...)
	}
	return true
}

// keyedArgs returns the first element index of each group in a slog call: the
// context, the level and the message, then each key and the value it labels.
// slog reads its trailing arguments as pairs, and a line break between a key
// and its value separates a label from the thing it names.
func keyedArgs(pass *analysis.Pass, call *ast.CallExpr) (starts []int, ok bool) {
	if call == nil {
		return nil, false
	}
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return nil, false
	}
	fn, isFn := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !isFn || fn.Pkg() == nil || fn.Pkg().Path() != "log/slog" {
		return nil, false
	}
	var head int // the arguments ahead of the first key
	switch fn.Name() {
	case "Debug", "Info", "Warn", "Error", "Group":
		head = 1
	case "DebugContext", "InfoContext", "WarnContext", "ErrorContext":
		head = 2
	case "Log":
		head = 3
	case "With":
		head = 0
	default:
		return nil, false // LogAttrs takes Attrs, which are whole values
	}
	if head > 0 {
		starts = append(starts, 0)
	}
	for i := head; i < len(call.Args); i += 2 {
		starts = append(starts, i)
	}
	return starts, len(starts) > 1
}

// packer is the state one list is laid out from: the text of each element, the
// indent every line begins with, and the width the lines must fit.
type packer struct {
	indent      []byte
	texts       [][]byte
	starts      []int    // the first element of each group; starts[0] is 0
	closeIndent []byte   // the indent the closer sits at
	solo        []bool   // the element takes a line of its own
	spans       []bool   // the element is written across lines
	trail       [][]byte // the comment that follows an element on its line
	oneGroup    bool     // each group takes a line of its own
	spread      bool     // the last element is followed by an ellipsis
	limit       int
}

// alone reports whether the line holding groups i through j-1 must hold only
// group i, because an element of it spans lines.
func (pk *packer) alone(i, j int) bool {
	if j <= i+1 {
		return false
	}
	for _, k := range pk.starts[i:j] {
		if pk.solo[k] {
			return true
		}
	}
	return false
}

// firstLine returns text up to its first newline, which is the line that has
// to fit the width.
func firstLine(text []byte) []byte {
	if before, _, ok := bytes.Cut(text, []byte{'\n'}); ok {
		return before
	}
	return text
}

// last returns the element index just past group j, or the element count when
// j is the last group.
func (pk *packer) last(j int) int {
	if j == len(pk.starts) {
		return len(pk.texts)
	}
	return pk.starts[j]
}

// line returns the line holding groups i through j-1, indented and ending with
// a comma.
func (pk *packer) line(i, j int) []byte {
	var (
		out = slices.Clone(pk.indent)
		lo  = pk.starts[i]
		hi  = pk.last(j)
	)
	for k := lo; k < hi; k++ {
		if k > lo {
			out = append(out, ", "...)
		}
		out = append(out, pk.texts[k]...)
		if hi == len(pk.texts) && pk.spread && k == hi-1 {
			out = append(out, "..."...)
		}
		if pk.trail[k] != nil {
			// The comma comes before the comment, which ends the
			// line.
			out = append(out, ", "...)
			out = append(out, pk.trail[k]...)
		}
	}
	if pk.trail[hi-1] != nil {
		return out // the comment took the comma
	}
	return append(out, ',')
}

// width returns the width of the line holding groups i through j-1. The width
// is linelen's, since a tab counts for more than one column, and it covers the
// first line only: the lines inside a spanning element are the element's own.
func (pk *packer) width(i, j int) int {
	return linelen.Width(firstLine(pk.line(i, j)))
}

// cheapest returns where each line ends in the layout with the least
// raggedness: the sum of the squared space left at the end of every line. A
// layout holds two lines or more, since a list that fits on one line is the
// join rule's work.
func (pk *packer) cheapest() (next []int, ok bool) {
	var (
		n     = len(pk.starts)
		cost  = make([]int, n+1)
		unset = -1
	)
	next = make([]int, n+1)
	for i := range cost {
		cost[i] = unset
	}
	cost[n] = 0
	for i := n - 1; i >= 0; i-- {
		for j := i + 1; j <= n; j++ {
			if i == 0 && j == n {
				continue // one line is the join rule's shape
			}
			if pk.oneGroup && j > i+1 {
				break // a group takes a line of its own
			}
			if pk.alone(i, j) {
				break // a spanning element leaves no room on its line
			}
			w := pk.width(i, j)
			if w > pk.limit {
				break // a longer line is only wider
			}
			if cost[j] == unset {
				continue
			}
			slack := pk.limit - w
			c := slack*slack + cost[j]
			if cost[i] == unset || c < cost[i] {
				cost[i], next[i] = c, j
			}
		}
	}
	return next, cost[0] != unset
}

// layout renders the lines the plan breaks at, each beginning with the indent
// and each ending with a comma but the last, whose comma the source keeps.
func (pk *packer) layout(next []int) layoutFill {
	var (
		out           []byte
		breaks        []int
		shared, alone bool
		n             = len(pk.starts)
	)
	for i := 0; i < n; {
		var (
			j    = next[i]
			line = pk.line(i, j)
		)
		// The edit begins just after the opener, so every line of the
		// layout starts on a line of its own.
		out = append(out, '\n')
		out = append(out, line...)
		if j < n {
			breaks = append(breaks, pk.starts[j])
		}
		if j > i+1 {
			shared = true
		}
		if j == i+1 && pk.spans[pk.starts[i]] {
			alone = true
		}
		i = j
	}
	// The closer takes a line of its own at the opener's indent.
	out = append(out, '\n')
	out = append(out, pk.closeIndent...)
	return layoutFill{
		text:   out,
		breaks: breaks,
		lines:  len(breaks) + 1,
		shared: shared,
		alone:  alone,
	}
}

// breaks returns the element index at which each line of the list's current
// layout begins, after the first.
func breaks(l *layout, p pair) (out []int) {
	for i := 1; i < len(p.elts); i++ {
		if l.line(p.elts[i].Pos()) != l.line(p.elts[i-1].End()) {
			out = append(out, i)
		}
	}
	return out
}

// render prints one expression.
func render(pass *analysis.Pass, e ast.Expr) ([]byte, bool) {
	var buf bytes.Buffer
	if err := format.Node(&buf, pass.Fset, e); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}
