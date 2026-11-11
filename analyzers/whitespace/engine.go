package whitespace

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"slices"
	"strings"
)

// file is one parsed source file and the line table the rules edit.
type file struct {
	fset    *token.FileSet
	tokFile *token.File
	syntax  *ast.File
}

// Format applies the spacing rules to one Go source file, named for a
// diagnostic.
func Format(name string, src []byte) ([]byte, error) { return space(name, src) }

// space applies the spacing rules to one Go source file and returns the file
// gofmt leaves.
func space(name string, src []byte) ([]byte, error) {
	fset, syntax, err := parse(name, src)
	if err != nil {
		return nil, err
	}
	if ast.IsGenerated(syntax) {
		return src, nil
	}
	f := &file{fset, fset.File(syntax.FileStart), syntax}
	methodSpacing(f)
	commentSpacing(f)
	out, err := canonical(name, fset, syntax)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// printerNormalizeNumbers is the mode bit go/format sets to have go/printer
// rewrite numeric literals to their canonical form. go/printer does not export
// it. The rules start from gofmt's output, so it is named here the way
// go/format names it.
const printerNormalizeNumbers = 1 << 30

var printerCfg = printer.Config{
	Mode:     printer.UseSpaces | printer.TabIndent | printerNormalizeNumbers,
	Tabwidth: 8,
}

// parse returns the parsed file and the file set it belongs to.
func parse(name string, src []byte) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	syntax, err := parser.ParseFile(
		fset, name, src,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, nil, err
	}
	return fset, syntax, nil
}

// canonical prints the file the way gofmt leaves it: imports sorted, then the
// printer's own form applied twice so the result is a fixed point.
func canonical(name string, fset *token.FileSet, syntax *ast.File) (string, error) {
	var printed string
	for range 2 {
		ast.SortImports(fset, syntax)
		var buf strings.Builder
		if err := printerCfg.Fprint(&buf, fset, syntax); err != nil {
			return "", err
		}
		printed = buf.String()
		var err error
		if fset, syntax, err = parse(name, []byte(printed)); err != nil {
			return "", err
		}
	}
	return printed, nil
}

// lineAt returns the unadjusted line number of pos, so a //line directive does
// not move it.
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

// firstDiff returns the offset of the first line that differs between the two
// sources, so a diagnostic points at the declaration that changes.
func firstDiff(a, b []byte) (off int) {
	var (
		linesA = bytes.Split(a, []byte("\n"))
		linesB = bytes.Split(b, []byte("\n"))
	)
	for i := range linesA {
		if i >= len(linesB) || !bytes.Equal(linesA[i], linesB[i]) {
			return off
		}
		off += len(linesA[i]) + 1
	}
	return 0
}
