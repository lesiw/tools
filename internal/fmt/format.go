package fmt

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// Format applies the layout rules to one Go source file. The rules run once,
// and the result is the file gofmt leaves. Format renders the input twice to
// bring it to the printer's own form, and it renders the result twice more.
//
// Each decision is measured against a rendering of the file as it stands. No
// decision makes a later one wrong. A join is kept only when the line it
// produces fits.
func Format(src []byte) ([]byte, error) {
	return apply("<standard input>", src)
}

// FormatFile is Format for a file named on a command line. A parse error then
// names the file it came from.
func FormatFile(name string, src []byte) ([]byte, error) {
	return apply(name, src)
}

// apply runs the rules over src in table order and canonicalizes the result.
func apply(name string, src []byte) ([]byte, error) {
	fset, file, err := prepare(name, src)
	if err != nil {
		return nil, err
	}
	r := newRenderer(fset, file)
	for _, rule := range rules() {
		rule(r)
	}
	_, _, out, err := canonical(name, fset, file)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// prepare parses src and returns the file the rules measure.
func prepare(name string, src []byte) (*token.FileSet, *ast.File, error) {
	fset, file, err := parse(name, src)
	if err != nil {
		return nil, nil, err
	}
	fset, file, _, err = canonical(name, fset, file)
	if err != nil {
		return nil, nil, err
	}
	return fset, file, nil
}

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

// canonical returns the file as go/printer renders it, parsed again. It also
// returns that text. The rules measure a file against the render, so the file
// they shape has to be one the printer produces.
//
// It renders twice. The first render brings the file to the printer's own
// form. A one-line body under a multi-line header comes back expanded, and the
// rules need that text to measure. The second render reads a file that already
// holds the boundaries the rules inserted. A boundary can end the printer's
// alignment section early, so the render that follows drops padding the
// previous one had. The second render is then the one the printer reproduces.
// It is also the one gofmt leaves, and the formatter's output has to be that,
// because it runs last in the go.fmt chain.
//
// Two renders, not a loop. A third render would mean a shape no fixture has
// produced, and TestFormatProperties would say so.
func canonical(name string, fset *token.FileSet, file *ast.File) (*token.FileSet, *ast.File, string, error) {
	var (
		printed string
		err     error
	)
	for range 2 {
		// gofmt sorts import groups. A file with its imports out of
		// order is not the file gofmt leaves.
		ast.SortImports(fset, file)
		if printed, err = printNode(fset, file); err != nil {
			return nil, nil, "", err
		}
		if fset, file, err = parse(name, []byte(printed)); err != nil {
			return nil, nil, "", err
		}
	}
	return fset, file, printed, nil
}

// newRenderer returns the renderer for a parsed file.
func newRenderer(fset *token.FileSet, file *ast.File) *renderer {
	return &renderer{fset: fset, tokFile: fset.File(file.Pos()), file: file}
}

// rules returns the layout rules in the order their inputs are ready. Each
// rule is a whole pass, and a rule may not assume the ones after it have run.
func rules() []func(*renderer) {
	return []func(*renderer){
		collapseSignatures,
		collapseSingleDecls,
		layoutLiterals,
		collapseDelimited,
		collapseWrappedLines,
		collapseHeaders,
		// Spacing runs after every rule that changes how many lines a
		// declaration occupies. Whether a declaration is multi-line
		// decides whether it needs blank lines around it.
		methodSpacing,
		commentSpacing,
		// alignClosers measures indents, so it runs after every rule
		// that moves a line.
		alignClosers,
	}
}
