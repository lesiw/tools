// Command fmt formats Go source read from standard input and writes
// the result to standard output, applying stricter layout rules than
// gofmt: method spacing, paired-closer merging, comment spacing,
// paren alignment, bin-packing, and line collapsing. Run it after
// gofmt and goimports, per the go.fmt convention. Type-aware rules
// live in the analyzer suite, not here.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
)

var reflow = flag.Bool(
	"reflow", false, "reflow doc comments to fill line width",
)

func main() {
	cfg.Flags(flag.CommandLine)
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	out, err := formatSrc(src, *reflow)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

// formatSrc applies the layout rules to one Go source file: parse once,
// reshape the tree and its line table, render once.
func formatSrc(src []byte, reflow bool) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "stdin.go", src,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, err
	}
	layout(fset, fset.File(file.Pos()), file, reflow)
	printed, err := printNode(fset, file)
	if err != nil {
		return nil, err
	}
	return []byte(printed), nil
}

// layout runs the rules over the parsed file in the order each one's
// input is ready. A rule that must see rendered text takes a snapshot,
// which carries every line boundary its predecessors set, so the pass
// order alone settles the layout.
func layout(fset *token.FileSet, tokFile *token.File, file *ast.File, reflow bool) {
	snap := func() *snapshot { return takeSnapshot(fset, tokFile, file) }
	collapseSignatures(tokFile, file)
	expandOneLineBodies(fset, tokFile, file, snap())
	methodSpacing(tokFile, file)
	mergePairedClosers(tokFile, file)
	collapseSingleDecls(tokFile, file)
	layoutLiterals(fset, tokFile, file, snap())
	collapseDelimited(fset, tokFile, file, snap())
	expandDelimited(fset, tokFile, file, snap())
	alignClosers(fset, tokFile, file, snap())
	collapseWrappedLines(tokFile, file, snap())
	liftOverlongComments(tokFile, file, snap())
	collapseHeaders(tokFile, file, snap())
	commentSpacing(tokFile, file)
	splitSeams(tokFile, file, snap())
	if reflow {
		reflowComments(fset, tokFile, file, snap())
	}
}
