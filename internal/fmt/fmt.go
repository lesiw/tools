// Package fmt formats Go source under stricter rules than gofmt. It joins a
// wrapped call or expression onto one line when the result fits. It joins a
// declaration block holding one spec, a cuddled brace pair, a signature, and a
// control-flow header. It sets the blank lines between declarations and before
// doc comments.
//
// Fmt never breaks a line to fit the width. A long line stays long, and the
// author decides how to shorten it. Fmt never turns a short line into a long
// one.
//
// Fmt does move a closer onto a line of its own when its opener sits at a
// different indent. A bracket pair belongs at one indent, and go/printer
// indents a closer on its own line to match the opener.
//
// Each rule runs once, so running fmt again changes nothing.
//
// go/printer expands one shape on its own. A one-line body under a multi-line
// header comes back on several lines, so the output can hold more lines than
// the input did. The printer does that, not fmt.
//
// Fmt measures each line against go/printer's rendering of the file. The width
// limit is the one the linelen analyzer reports.
//
// The command takes gofmt's arguments. It reads standard input when there are
// no arguments, and files and directories otherwise. The -l flag lists what
// differs, and -w rewrites in place. It skips a walked file whose name begins
// with a dot, and it leaves a generated file alone.
package fmt

import (
	"bytes"
	"context"
	"errors"
	"flag"
	stdfmt "fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"
)

var list, write *bool

// flags registers the width flags on fs.
func flags(fs *flag.FlagSet) { cfg.Flags(fs) }

// register sets up the command's flags on fs.
func register(fs *flag.FlagSet) {
	list = fs.Bool("l", false, "list files whose formatting differs")
	write = fs.Bool("w", false, "write result to (source) file")
	flags(fs)
}

// Main runs the command with args and returns the exit status: 2 when it was
// called wrongly, 1 when it ran and something failed.
func Main(args []string) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	register(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := run(fs.Args()); err != nil {
		stdfmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(args []string) error {
	if len(args) > 0 {
		return formatFiles(args)
	}
	if *write {
		return stdfmt.Errorf("cannot use -w with standard input")
	}
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return stdfmt.Errorf("read stdin: %w", err)
	}
	// A generated file belongs to its generator, so it passes through as
	// it is. -l names what differs, and a generated file never does.
	if generated(src) {
		if *list {
			return nil
		}
		_, err = os.Stdout.Write(src)
		return err
	}
	out, err := Format(src)
	if err != nil {
		return err
	}
	if *list {
		if !bytes.Equal(out, src) {
			stdfmt.Println("<standard input>")
		}
		return nil
	}
	_, err = os.Stdout.Write(out)
	return err
}

// generated reports whether src says it was generated.
func generated(src []byte) bool {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src,
		parser.ParseComments|parser.PackageClauseOnly,
	)
	if err != nil {
		return false
	}
	return ast.IsGenerated(file)
}

// formatFiles lays out the files and directories named on the command line.
func formatFiles(args []string) error {
	files, err := gather(args)
	if err != nil {
		return err
	}
	// Every file is formatted at once, bounded by the memory a parse tree
	// costs: gofmt limits in-flight work to 2 MiB times the number of
	// threads for the same reason, and a file bigger than the whole budget
	// takes the budget. Output is written in the order the files were
	// named, not the order they finished.
	var (
		budget = maxWeight()
		sem    = semaphore.NewWeighted(budget)
		listed = make([]string, len(files))
		writes = make([][]byte, len(files))
		errs   = make([]error, len(files))
		group  sync.WaitGroup
	)
	for i, name := range files {
		weight := int64(1)
		if info, err := os.Stat(name); err == nil {
			weight = min(max(1, info.Size()), budget)
		}
		// Acquire fails only when its context is done, and this one
		// never is; the call is for its blocking.
		_ = sem.Acquire(context.Background(), weight)
		group.Go(func() {
			defer sem.Release(weight)
			listed[i], writes[i], errs[i] = formatFile(name)
		})
	}
	group.Wait()
	for i := range files {
		if listed[i] != "" && *list {
			stdfmt.Println(listed[i])
		}
		if len(writes[i]) > 0 {
			if _, err := os.Stdout.Write(writes[i]); err != nil {
				return err
			}
		}
	}
	return errors.Join(errs...)
}

// maxWeight is the in-flight work one run allows: 2 MiB times the number of
// threads, the bound gofmt uses.
func maxWeight() int64 {
	return (2 << 20) * int64(runtime.GOMAXPROCS(0))
}

// formatFile lays out one file. It returns the name to list, if the file
// differs, and the text to write, when neither -l nor -w was given.
func formatFile(name string) (listed string, out []byte, err error) {
	src, err := os.ReadFile(name)
	if err != nil {
		return "", nil, err
	}
	// A generated file belongs to its generator, so it is not read as Go
	// at all: a file the parser rejects still leaves this command alone.
	if generated(src) {
		// A generated file belongs to its generator, so it passes
		// through as it is.
		if !*list && !*write {
			return "", src, nil
		}
		return "", nil, nil
	}
	formatted, err := FormatFile(name, src)
	if err != nil {
		return "", nil, err
	}
	if !*list && !*write {
		return "", formatted, nil
	}
	if bytes.Equal(formatted, src) {
		return "", nil, nil
	}
	if *write {
		return name, nil, os.WriteFile(name, formatted, 0o644)
	}
	return name, nil, nil
}

// gather resolves arguments to the files to format: named files as given, and
// .go files found under named directories.
func gather(args []string) (files []string, err error) {
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, arg)
			continue
		}
		walk := func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if isGoFilename(d.Name()) {
				files = append(files, path)
			}
			return nil
		}
		if err := filepath.WalkDir(arg, walk); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// isGoFilename reports whether a file name is a .go file that does not begin
// with a dot.
func isGoFilename(name string) bool {
	return !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}
