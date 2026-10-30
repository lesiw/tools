// Package reflow fills comment prose to the width limit. It also lifts a
// trailing comment that is too long for its line above the code it trails.
//
// It is a separate command from fmt because it moves the line breaks inside
// comments. Reflow joins a paragraph's lines and breaks them again at the
// width limit. An author who breaks those lines on purpose loses them.
package reflow

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// Main reads Go source from stdin and writes the reflowed source to stdout. It
// returns the exit status. That is 2 when it was called wrongly, and 1 when it
// ran and something failed.
func Main(args []string) int {
	fs := flag.NewFlagSet("reflow", flag.ContinueOnError)
	flags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := run(fs.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if len(fs.Args()) > 0 {
			return 2
		}
		return 1
	}
	return 0
}

func run(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("reflow reads standard input, not files")
	}
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	out, err := Reflow(src)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

// Reflow rewrites comment prose. A doc comment is filled to the width limit,
// and a trailing comment too long to leave on its line is lifted above the
// code it trails. The file is rendered through go/printer, so gofmt's
// normalizations come with it. The rules themselves change only the comments.
func Reflow(src []byte) ([]byte, error) {
	const name = "<standard input>"
	fset, file, err := parse(name, src)
	if err != nil {
		return nil, err
	}
	fset, file, _, err = canonical(name, fset, file)
	if err != nil {
		return nil, err
	}
	r := newRenderer(fset, file)
	liftOverlongComments(r)
	reflowComments(r)
	_, _, out, err := canonical(name, fset, file)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}
