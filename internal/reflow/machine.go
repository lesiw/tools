package reflow

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"slices"
	"strings"

	"lesiw.io/tools/analyzers/linelen"
)

// flags registers the width flags on fs.
func flags(fs *flag.FlagSet) { linelen.Flags(fs) }

// printerNormalizeNumbers is the mode bit go/format sets to have go/printer
// rewrite numeric literals to their canonical form. The output has to be the
// same text gofmt leaves, so this copy names it the way go/format does.
const printerNormalizeNumbers = 1 << 30

var printerCfg = printer.Config{
	Mode:     printer.UseSpaces | printer.TabIndent | printerNormalizeNumbers,
	Tabwidth: 8,
}

// renderer is the file a rule edits together with the printer's account of it.
type renderer struct {
	fset    *token.FileSet
	tokFile *token.File
	file    *ast.File
}

// newRenderer returns the renderer for a parsed file.
func newRenderer(fset *token.FileSet, file *ast.File) *renderer {
	return &renderer{fset: fset, tokFile: fset.File(file.Pos()), file: file}
}

// snapshot is the file as go/printer renders it, split into lines, with the
// line table that produced it. A rule that edits the table while it iterates
// keeps reading the lines it measured, so the table is copied.
type snapshot struct {
	lines  [][]byte
	starts []int
	base   int
}

func (r *renderer) snapshot() *snapshot {
	var buf bytes.Buffer
	if err := printerCfg.Fprint(&buf, r.fset, r.file); err != nil {
		return new(snapshot)
	}
	return &snapshot{
		lines:  bytes.Split(buf.Bytes(), []byte("\n")),
		starts: slices.Clone(r.tokFile.Lines()),
		base:   r.tokFile.Base(),
	}
}

// line returns the line number pos held when the snapshot was taken.
func (s *snapshot) line(pos token.Pos) int {
	i, exact := slices.BinarySearch(s.starts, int(pos)-s.base)
	if exact {
		return i + 1
	}
	return i
}

// text returns the rendered text of a line, or nil when the line is out of
// range.
func (s *snapshot) text(line int) []byte {
	if line < 1 || line > len(s.lines) {
		return nil
	}
	return s.lines[line-1]
}

// fits reports whether line may stay on one line, trailing comment included.
func fits(line []byte) bool { return linelen.CheckLine(line) }

// lineAt returns the line pos is written on, ignoring any //line directive.
func lineAt(tokFile *token.File, pos token.Pos) int {
	return tokFile.PositionFor(pos, false).Line
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

// extractIndent returns the leading whitespace of line.
func extractIndent(line []byte) []byte {
	for i, b := range line {
		if b != '\t' && b != ' ' {
			return line[:i]
		}
	}
	return line
}

// canonical returns the file as go/printer renders it, parsed again. It also
// returns that text, so the output is the text gofmt leaves.
func canonical(name string, fset *token.FileSet, file *ast.File) (*token.FileSet, *ast.File, string, error) {
	var (
		printed string
		err     error
	)
	for range 2 {
		ast.SortImports(fset, file)
		var buf strings.Builder
		if err = printerCfg.Fprint(&buf, fset, file); err != nil {
			return nil, nil, "", err
		}
		printed = buf.String()
		if fset, file, err = parse(name, []byte(printed)); err != nil {
			return nil, nil, "", err
		}
	}
	return fset, file, printed, nil
}

// parse parses src as Go source.
func parse(name string, src []byte) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, nil, err
	}
	return fset, file, nil
}
