// Package linelen provides an analysis.Analyzer for line length,
// and the line-length rules themselves for other tools to share.
package linelen

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/scanner"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/signature"
)

// Config is the line-length policy. Len at zero means unlimited; a driver
// that wants a limit sets it before analysis starts.
var Config = struct {
	Len int // maximum line width in visual columns; zero means no limit
	Tab int // visual width of a tab
}{0, 4}

// Flags registers the len and tab flags on fs, so every command sharing the
// rules exposes identical knobs. A flag registered with the value Config
// already holds leaves that value in place until the flag is set.
func Flags(fs *flag.FlagSet) {
	fs.IntVar(&Config.Len, "len", Config.Len, "maximum line length")
	fs.IntVar(&Config.Tab, "tab", Config.Tab, "tab width in spaces")
}

// Width returns the visual width of line: a tab counts Tab columns,
// every other rune one. A trailing carriage return does not count.
func Width(line []byte) (width int) {
	line = bytes.TrimSuffix(line, []byte("\r"))
	for _, r := range string(line) {
		if r == '\t' {
			width += cmp.Or(Config.Tab, 4)
		} else {
			width++
		}
	}
	return width
}

// Line describes a line of a source file.
type Line struct {
	N     int       // 1-based line number the file is written on
	Width int       // visual width in columns
	Pos   token.Pos // position of the line's first byte
}

// span is a half-open byte range within a line.
type span struct {
	pos int // inclusive offset of the first byte
	end int // exclusive offset of the last byte
}

// CheckLine reports whether line is within the limit, or more than
// half of it is one continuous string literal or URL. It is lexical: a
// line that begins inside a multiline literal is not seen.
func CheckLine(line []byte) bool {
	if Config.Len <= 0 {
		return true
	}
	return Width(line) <= Config.Len || continuous(line, scanLits(line))
}

// CheckFile returns f's overlong lines: every line of src whose width
// exceeds the limit, which f measures, and which is not continuous.
// Generated files are the caller's to skip.
func CheckFile(fset *token.FileSet, f *ast.File, src []byte) (long []Line) {
	if Config.Len <= 0 {
		return nil
	}
	var (
		n, pos   int
		measured = checked(fset, f, src)
		file     = fset.File(f.FileStart)
	)
	for line := range bytes.SplitSeq(src, []byte("\n")) {
		n++
		width := Width(line)
		if measured(n, line) && width > Config.Len {
			long = append(long, Line{N: n, Width: width, Pos: file.Pos(pos)})
		}
		pos += len(line) + 1 // len("\n")
	}
	return long
}

// explained reports whether the signature text on a line accounts for its
// width: the line is within the limit once that text is removed. A signature
// has no second line to move to, so the limit has nothing to say about it. A
// line the rest of the statement made long is still the author's to wrap.
func explained(line []byte, spans []span) bool {
	if len(spans) == 0 {
		return false
	}
	slices.SortFunc(spans, byOffset)
	var (
		kept []byte
		end  int
	)
	for _, s := range spans {
		lo, hi := max(s.pos, end), min(s.end, len(line))
		if lo >= hi {
			continue
		}
		kept = append(kept, line[end:lo]...)
		end = hi
	}
	kept = append(kept, line[end:]...)
	return Width(kept) <= Config.Len
}

// byOffset orders two spans of one line by where they start.
func byOffset(a, b span) int { return cmp.Compare(a.pos, b.pos) }

// continuous reports whether one continuous string literal or URL takes
// up more than half of the line's width, ignoring indentation.
func continuous(line []byte, lits []span) bool {
	var (
		body    = bytes.TrimLeft(line, " \t")
		shifted = shiftSpans(lits, len(line)-len(body))
		width   = Width(body)
	)
	for _, s := range shifted {
		if dominant(body, s, width) {
			return true
		}
	}
	for _, u := range urlSpans(body) {
		if !within(shifted, u) && dominant(body, u, width) {
			return true
		}
	}
	return false
}

// shiftSpans moves spans left by shift, the width of a line's indent.
func shiftSpans(spans []span, shift int) (out []span) {
	if shift == 0 {
		return spans
	}
	for _, s := range spans {
		out = append(out, span{
			pos: max(s.pos-shift, 0),
			end: max(s.end-shift, 0),
		})
	}
	return out
}

// dominant reports whether s is more than half of the limit and more than
// half of a line of the given width.
func dominant(body []byte, s span, width int) bool {
	lo, hi := max(s.pos, 0), min(s.end, len(body))
	if lo >= hi {
		return false
	}
	if 2*Width(body[lo:hi]) <= width {
		return false
	}
	start := Width(body[:lo])
	if start >= Config.Len {
		return false
	}
	return 2*(min(Width(body[:hi]), Config.Len)-start) > Config.Len
}

// within reports whether s lies inside one of spans.
func within(spans []span, s span) bool {
	for _, l := range spans {
		if s.pos >= l.pos && s.end <= l.end {
			return true
		}
	}
	return false
}

// urlSpans returns the byte ranges of whitespace-delimited words
// that carry a URL scheme.
func urlSpans(line []byte) (out []span) {
	for i := 0; i < len(line); {
		for i < len(line) && isSpace(line[i]) {
			i++
		}
		start := i
		for i < len(line) && !isSpace(line[i]) {
			i++
		}
		if bytes.Contains(line[start:i], []byte("://")) {
			out = append(out, span{pos: start, end: i})
		}
	}
	return out
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' }

// scanLits returns the string literals in line as byte ranges. A raw
// string that runs past the line contributes its part; a line that
// begins inside one is not seen.
func scanLits(line []byte) (out []span) {
	var (
		fset = token.NewFileSet()
		file = fset.AddFile("", fset.Base(), len(line))
		s    scanner.Scanner
	)
	s.Init(file, line, nil, 0)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.STRING {
			continue
		}
		lo := file.Offset(pos)
		out = append(out, span{pos: lo, end: lo + len(lit)})
		if lit[0] == '`' && (len(lit) < 2 || lit[len(lit)-1] != '`') {
			break
		}
	}
	return out
}

// writtenLine returns the line pos is written on, ignoring //line
// directives.
func writtenLine(fset *token.FileSet, pos token.Pos) int {
	return fset.PositionFor(pos, false).Line
}

// writtenLines returns the first and last line pos and end are written on,
// ignoring //line directives. The line table is indexed by the lines the file
// is written on, and a directive can name a line past its end.
func writtenLines(fset *token.FileSet, pos, end token.Pos) (first, last int) {
	return writtenLine(fset, pos), writtenLine(fset, end)
}

// astLits maps a line number to the parts of string literals on it,
// as byte ranges within the line. A literal that spans lines
// contributes the part on each line.
func astLits(fset *token.FileSet, f *ast.File) map[int][]span {
	out := make(map[int][]span)
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		file := fset.File(lit.Pos())
		if file == nil {
			return true
		}
		pos, end := file.Offset(lit.Pos()), file.Offset(lit.End())
		first, last := writtenLines(fset, lit.Pos(), lit.End())
		for line := first; line <= last; line++ {
			var (
				start = file.Offset(file.LineStart(line))
				from  = max(pos, start)
				to    = end
			)
			if line < file.LineCount() {
				to = min(to, file.Offset(file.LineStart(line+1)))
			}
			if to > from {
				out[line] = append(
					out[line],
					span{pos: from - start, end: to - start},
				)
			}
		}
		return true
	})
	return out
}

// checked returns a predicate reporting whether the limit applies to a line of
// f, given the line's text. An import declaration is not measured, and neither
// is a line whose width the signature text on it accounts for: a signature has
// no second line to move to, so the limit could only ask for a wrap that the
// join rule undoes. A line the rest of the statement makes long is still
// measured, and so is the line a body's first statement starts on, because the
// statement can move down.
func checked(fset *token.FileSet, f *ast.File, src []byte) func(line int, text []byte) bool {
	var (
		skip      = make(map[int]struct{})
		sigs      = make(map[int][]span)
		lits      = astLits(fset, f)
		markLines = func(first, last int) {
			for line := first; line <= last; line++ {
				skip[line] = struct{}{}
			}
		}
	)
	markExpectedOutput(fset, f, src, markLines)
	ast.Inspect(f, func(n ast.Node) bool {
		if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
			markLines(writtenLines(fset, decl.Pos(), decl.End()))
		}
		return true
	})
	for _, sig := range signature.Spans(f) {
		first, last := writtenLines(fset, sig.Pos(), sig.End())
		if start := sig.BodyStart(); start.IsValid() {
			last = min(last, writtenLine(fset, start)-1)
		}
		file := fset.File(sig.Pos())
		lo, hi := file.Offset(sig.Pos()), file.Offset(sig.End())
		for line := first; line <= last; line++ {
			var (
				at  = file.Offset(file.LineStart(line))
				end = file.Size()
			)
			if line < file.LineCount() {
				end = file.Offset(file.LineStart(line + 1))
			}
			from, to := max(lo, at), min(hi, end)
			if to > from {
				sigs[line] = append(sigs[line], span{
					pos: from - at,
					end: to - at,
				})
			}
		}
	}
	return func(line int, text []byte) bool {
		if _, ok := skip[line]; ok {
			return false
		}
		if explained(text, sigs[line]) {
			return false
		}
		return !continuous(text, lits[line])
	}
}

// markExpectedOutput marks the lines of the comment holding an example's
// expected output, which a test compares. go/doc reports which functions are
// examples and which have output; it leaves the test-file check to its caller,
// because only a test file's examples run.
func markExpectedOutput(fset *token.FileSet, f *ast.File, src []byte, mark func(first, last int)) {
	if !strings.HasSuffix(fset.File(f.FileStart).Name(), "_test.go") {
		return
	}
	// doc.Example.Name is the name with its prefix stripped.
	withOutput := make(map[string]struct{})
	for _, ex := range doc.Examples(f) {
		if ex.Output != "" {
			withOutput["Example"+ex.Name] = struct{}{}
		}
	}
	for _, decl := range f.Decls {
		node, ok := decl.(*ast.FuncDecl)
		// A method may share an example's name.
		if !ok || node.Recv != nil {
			continue
		}
		if _, ok := withOutput[node.Name.Name]; !ok {
			continue
		}
		// go/doc reads the output from the body's last comment, so that
		// is the comment this covers.
		for _, cg := range slices.Backward(f.Comments) {
			if cg.End() > node.Body.End() || cg.Pos() < node.Body.Pos() {
				continue
			}
			// A line that carries a statement is measured; the
			// comment can move down instead.
			first, last := writtenLines(fset, cg.Pos(), cg.End())
			if !leadsLine(src, fset, cg.Pos()) {
				first++
			}
			if first <= last {
				mark(first, last)
			}
			break
		}
	}
}

// leadsLine reports whether only indentation comes before pos on its line.
func leadsLine(src []byte, fset *token.FileSet, pos token.Pos) bool {
	var (
		file  = fset.File(pos)
		start = file.Offset(file.LineStart(writtenLine(fset, pos)))
	)
	return len(bytes.TrimSpace(src[start:file.Offset(pos)])) == 0
}

var Analyzer = &analysis.Analyzer{
	Name: "linelen",
	Doc:  "reports lines longer than the specified character limit",
	Run:  run,
}

func init() {
	Flags(&Analyzer.Flags)
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		file := pass.Fset.File(f.FileStart)
		src, err := pass.ReadFile(file.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		for _, l := range CheckFile(pass.Fset, f, src) {
			pass.Reportf(l.Pos, "line is %d characters long, exceeds %d limit", l.Width, Config.Len)
		}
	}
	return nil, nil
}
