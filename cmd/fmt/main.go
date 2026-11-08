// Command fmt presents gofmt's interface over the analyzer suite.
//
//	go tool lesiw.io/tools/cmd/fmt [-l] [-w] [path ...]
//
// It takes gofmt's arguments and keeps gofmt's behavior: it reads standard
// input when no path is named, -l lists the files that differ, and -w rewrites
// them. What differs is the work: a path is handed to the fix suite, so the
// output is what lesiw.io/tools/cmd/fix produces rather than what one
// formatter pass produces.
//
// Standard input is the exception. A single file needs no package, so it is
// formatted in place by the rules that need no package, and the result is
// written to standard output.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/unwrap"
	"lesiw.io/tools/analyzers/whitespace"
	"lesiw.io/tools/internal/fix"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	var (
		list, write bool
		paths       []string
	)
	for _, arg := range args {
		switch {
		case arg == "-l":
			list = true
		case arg == "-w":
			write = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "fmt: unknown flag %s\n", arg)
			return 2
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		return stdin()
	}
	if write {
		if onlyFiles(paths) {
			// gofmt rewrites what it was given. The suite loads
			// packages, so a named file gets the layout rules in
			// place and the package-wide rules wait for a directory.
			return writeFiles(paths)
		}
		return fix.Main(patterns(paths))
	}
	if list {
		return listFiles(paths)
	}
	// gofmt prints the formatted source when neither -l nor -w is given.
	return stdinPaths(paths)
}

// format runs the rules that need no package: the joins, then the blank lines.
func format(name string, src []byte) ([]byte, error) {
	joined, err := unwrap.Format(name, src)
	if err != nil {
		return nil, err
	}
	return whitespace.Format(name, joined)
}

// stdin formats standard input with the layout rules and writes the result.
func stdin() int {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	out, err := format("<standard input>", src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	return 0
}

// stdinPaths prints the formatted form of each named file.
func stdinPaths(paths []string) int {
	files, err := goFiles(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
		out, err := format(name, src)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
		if _, err := os.Stdout.Write(out); err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
	}
	return 0
}

// listFiles prints the files the suite would change. A named file is checked
// on its own, since a single file needs no package; a directory is handed to
// the suite.
func listFiles(paths []string) int {
	var dirs, named []string
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			named = append(named, path)
			continue
		}
		dirs = append(dirs, path)
	}
	for _, name := range named {
		src, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
		out, err := format(name, src)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
		if !bytes.Equal(out, src) {
			fmt.Println(name)
		}
	}
	if len(dirs) == 0 {
		return 0
	}
	return listPackages(patterns(dirs))
}

// listPackages prints the files the suite would change under each pattern.
func listPackages(patterns []string) int {
	tool, cleanup, err := fix.Tool()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	defer cleanup()
	out, err := exec.Command("go",
		append([]string{"vet", "-vettool=" + tool, "-json"}, patterns...)...,
	).Output()
	if len(out) == 0 && err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	names, err := filesWithFixes(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fmt:", err)
		return 1
	}
	for _, name := range names {
		fmt.Println(name)
	}
	return 0
}

// filesWithFixes returns the distinct files named by a diagnostic that carries
// a fix, which is the set a fix run would rewrite.
func filesWithFixes(out []byte) ([]string, error) {
	var (
		names []string
		seen  = make(map[string]struct{})
		dec   = json.NewDecoder(bytes.NewReader(out))
	)
	for {
		var report map[string]map[string][]struct {
			Posn  string     `json:"posn"`
			Fixes []struct{} `json:"suggested_fixes"`
		}
		if err := dec.Decode(&report); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		for _, byAnalyzer := range report {
			for _, diags := range byAnalyzer {
				for _, d := range diags {
					if len(d.Fixes) == 0 {
						continue
					}
					name, _, _ := strings.Cut(d.Posn, ":")
					if _, ok := seen[name]; ok {
						continue
					}
					seen[name] = struct{}{}
					names = append(names, name)
				}
			}
		}
	}
	return names, nil
}

// onlyFiles reports whether every path names a file.
func onlyFiles(paths []string) bool {
	for _, path := range paths {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// writeFiles rewrites each named file with the layout rules.
func writeFiles(paths []string) int {
	for _, name := range paths {
		src, err := os.ReadFile(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
		out, err := format(name, src)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
		if bytes.Equal(out, src) {
			continue
		}
		if err := os.WriteFile(name, out, 0o666); err != nil {
			fmt.Fprintln(os.Stderr, "fmt:", err)
			return 1
		}
	}
	return 0
}

// patterns turns gofmt's paths into package patterns for the go command. A
// named file becomes the directory that holds it, since the suite loads
// packages rather than files.
func patterns(paths []string) (out []string) {
	seen := make(map[string]struct{})
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			path = filepath.Dir(path)
		}
		// A bare name is an import path to the go command, so a
		// relative directory is spelled from the working directory.
		path = relativePattern(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

// relativePattern spells a path the way the go command reads a directory
// pattern: relative to the working directory, and never as an import path.
func relativePattern(path string) string {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "./") {
		return path
	}
	if strings.HasPrefix(path, "../") {
		return path
	}
	return "./" + path
}

// goFiles returns the Go files a list of paths names.
func goFiles(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		walk := func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
				return err
			}
			files = append(files, p)
			return nil
		}
		if err := filepath.WalkDir(path, walk); err != nil {
			return nil, err
		}
	}
	return files, nil
}
