// Package linelen provides an analysis.Analyzer for line length,
// and the line-length rules themselves for other tools to share.
package linelen

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Config configures the line-length rules. The zero value uses the
// defaults: a 79 column limit and 4 column tabs.
type Config struct {
	Len int // maximum line width in visual columns
	Tab int // visual width of a tab
}

// len and tab resolve unset fields to the defaults, so cmp.Or
// appears here and nowhere else.
func (c Config) len() int { return cmp.Or(c.Len, 79) }
func (c Config) tab() int { return cmp.Or(c.Tab, 4) }

// Flags registers the len and tab flags on fs, bound to c, so every
// command sharing the rules exposes identical knobs.
func (c *Config) Flags(fs *flag.FlagSet) {
	fs.IntVar(&c.Len, "len", c.len(), "maximum line length")
	fs.IntVar(&c.Tab, "tab", c.tab(), "tab width in spaces")
}

// Width returns the visual width of line: a tab counts Tab columns,
// every other rune one. A trailing carriage return does not count.
func (c Config) Width(line []byte) (width int) {
	tab := c.tab()
	line = bytes.TrimSuffix(line, []byte("\r"))
	for _, r := range string(line) {
		if r == '\t' {
			width += tab
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
func (c Config) CheckLine(line []byte) bool {
	return c.Width(line) <= c.len() || c.continuous(line, scanLits(line))
}

// CheckFile returns f's overlong lines: every line of src whose width
// exceeds the limit, which f measures, and which is not continuous.
// Positions where wrapping is the author's layout, and generated
// files, are the caller's to skip.
func (c Config) CheckFile(fset *token.FileSet, f *ast.File, src []byte) (long []Line) {
	var (
		n, pos   int
		measured = checked(fset, f)
		file     = fset.File(f.FileStart)
		limit    = c.len()
		lits     = astLits(fset, f)
	)
	for line := range bytes.SplitSeq(src, []byte("\n")) {
		n++
		width := c.Width(line)
		if measured(n) && width > limit && !c.continuous(line, lits[n]) {
			long = append(long, Line{N: n, Width: width, Pos: file.Pos(pos)})
		}
		pos += len(line) + 1 // len("\n")
	}
	return long
}

// continuous reports whether one continuous string literal or URL takes
// up more than half of the line's width, ignoring indentation.
func (c Config) continuous(line []byte, lits []span) bool {
	var (
		body    = bytes.TrimLeft(line, " \t")
		shifted = shiftSpans(lits, len(line)-len(body))
		width   = c.Width(body)
	)
	for _, s := range shifted {
		if c.dominant(body, s, width) {
			return true
		}
	}
	for _, u := range urlSpans(body) {
		if !within(shifted, u) && c.dominant(body, u, width) {
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
func (c Config) dominant(body []byte, s span, width int) bool {
	lo, hi := max(s.pos, 0), min(s.end, len(body))
	if lo >= hi {
		return false
	}
	if 2*c.Width(body[lo:hi]) <= width {
		return false
	}
	start := c.Width(body[:lo])
	if start >= c.len() {
		return false
	}
	return 2*(min(c.Width(body[:hi]), c.len())-start) > c.len()
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

// writtenLines returns the first and last line pos and end are written on,
// ignoring //line directives. The line table is indexed by the lines the file
// is written on, and a directive can name a line past its end.
func writtenLines(fset *token.FileSet, pos, end token.Pos) (first, last int) {
	return fset.PositionFor(pos, false).Line, fset.PositionFor(end, false).Line
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
				out[line] = append(out[line],
					span{pos: from - start, end: to - start},
				)
			}
		}
		return true
	})
	return out
}

// checked returns a predicate reporting whether the limit applies to
// a line of f. It does not apply to import declarations, and to
// function, method, and interface method signatures, whose wrapping
// is the author's chosen layout; a function's body is measured.
func checked(fset *token.FileSet, f *ast.File) func(line int) bool {
	var (
		skip = make(map[int]struct{})
		mark = func(start, end token.Pos) {
			s, e := writtenLines(fset, start, end)
			for line := s; line <= e; line++ {
				skip[line] = struct{}{}
			}
		}
	)
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ImportSpec:
			mark(node.Pos(), node.Pos())
		case *ast.GenDecl:
			if node.Tok.String() == "import" {
				mark(node.Pos(), node.End())
			}
		case *ast.FuncDecl:
			// Function and method signatures may wrap across lines;
			// the wrap is the user's chosen layout. The body still
			// gets checked.
			end := node.Type.End()
			if node.Body != nil {
				end = node.Body.Lbrace
			}
			mark(node.Pos(), end)
		case *ast.InterfaceType:
			// Interface methods are *ast.Field entries whose Type
			// is *ast.FuncType. The whole field span is the
			// signature; let it wrap.
			if node.Methods == nil {
				return true
			}
			for _, field := range node.Methods.List {
				if _, ok := field.Type.(*ast.FuncType); !ok {
					continue
				}
				mark(field.Pos(), field.End())
			}
		}
		return true
	})
	return func(line int) bool {
		_, skipped := skip[line]
		return !skipped
	}
}

var (
	cfg      Config
	Analyzer = &analysis.Analyzer{
		Name: "linelen",
		Doc:  "reports lines longer than the specified character limit",
		Run:  run,
	}
)

func init() {
	cfg.Flags(&Analyzer.Flags)
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		if ast.IsGenerated(f) {
			continue
		}
		file := pass.Fset.File(f.FileStart)
		src, err := pass.ReadFile(file.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		for _, l := range cfg.CheckFile(pass.Fset, f, src) {
			pass.Reportf(l.Pos, "line is %d characters long, exceeds %d limit", l.Width, cfg.len())
		}
	}
	return nil, nil
}
