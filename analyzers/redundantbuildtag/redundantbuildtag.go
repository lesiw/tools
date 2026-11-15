// Package redundantbuildtag provides an analysis.Analyzer that reports a
// //go:build line the file name already implies.
//
// Go's build constraint documentation (go help buildconstraint) makes a file
// name that matches *_GOOS, *_GOARCH or *_GOOS_GOARCH part of the constraint:
// "the file is considered to have an implicit build constraint requiring those
// terms (in addition to any explicit constraints in the file)".
//
// The name and the line are combined with AND, and the build sets the tag the
// name gives wherever it uses the file. A condition that holds on that tag
// alone states nothing, and the file is built the same way without it.
//
// A compound condition goes too when the name settles it: //go:build linux &&
// amd64 is redundant in a file named a_linux_amd64.go. A tag the name leaves
// open stands, so //go:build linux && cgo stays in a_linux.go.
//
// The name's suffix is matched as a tag, so -tags=linux builds a_linux.go on
// Windows. The rule therefore says nothing about a tag the name does not
// spell: //go:build unix in a_linux.go and //go:build !unix in a_windows.go
// are left alone. A GOOS that implies another is not enough either, since
// -tags=android matches an android name on any system, where linux is unset.
//
// The fix removes the line, and the blank line after it when the line is the
// whole of its comment group. A // +build line beside it goes too: the build
// ignores those while a //go:build line is present, and gofmt would write the
// //go:build line back from one, so the redundancy would return. A file whose
// only constraint is a // +build line is left alone, since gofmt writes the
// //go:build line from it and the next run has something to report.
//
// The rule is adapted from revive's redundant-build-tag.
package redundantbuildtag

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "redundantbuildtag",
	Doc:  "reports a build constraint the file name implies",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		if err := check(pass, file, name); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// check reports the file's build constraint when the name already gives it.
func check(pass *analysis.Pass, file *ast.File, name string) error {
	comment, expr, drops := constraintLines(file)
	if expr == nil {
		return nil
	}
	if !always(expr, namedTags(filepath.Base(name))) {
		return nil
	}
	return report(pass, comment, drops)
}

// drop is one line the fix removes, and whether it is the whole of its
// comment group.
type drop struct {
	comment *ast.Comment
	alone   bool
}

// constraintLines returns the //go:build comment that constrains the file, its
// condition, and the lines the fix removes. The go command reads a constraint
// from the comments before the package clause and nowhere else.
func constraintLines(file *ast.File) (comment *ast.Comment, expr constraint.Expr, drops []drop) {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		alone := true
		for _, c := range group.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				continue
			}
			alone = false
		}
		for _, c := range group.List {
			switch {
			case constraint.IsPlusBuild(c.Text):
				drops = append(drops, drop{c, alone})
			case comment == nil && constraint.IsGoBuild(c.Text):
				x, err := constraint.Parse(c.Text)
				if err != nil {
					return nil, nil, nil
				}
				comment, expr = c, x
				drops = append(drops, drop{c, alone})
			}
		}
	}
	return comment, expr, drops
}

// always reports whether the condition holds in every build that can use the
// file. A tag the name does not give may be set or unset, so an expression
// holding one can go either way and the answer is no.
func always(expr constraint.Expr, named []string) bool {
	switch expr := expr.(type) {
	case *constraint.TagExpr:
		return slices.Contains(named, expr.Tag)
	case *constraint.AndExpr:
		return always(expr.X, named) && always(expr.Y, named)
	case *constraint.OrExpr:
		return always(expr.X, named) || always(expr.Y, named)
	}
	return false
}

// report reports the constraint and offers the edits that drop every line it
// is written on.
func report(pass *analysis.Pass, comment *ast.Comment, drops []drop) error {
	file := pass.Fset.File(comment.Pos())
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return fmt.Errorf("read %s: %w", file.Name(), err)
	}
	edits := make([]analysis.TextEdit, 0, len(drops))
	for _, d := range drops {
		edits = append(edits, removal(file, d, src))
	}
	pass.Report(analysis.Diagnostic{
		Pos:     comment.Pos(),
		End:     comment.End(),
		Message: "redundant build constraint",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "Drop the constraint",
			TextEdits: edits,
		}},
	})
	return nil
}

// removal returns the edit that drops one line of the constraint and its
// newline. When the line is the whole of its comment group, the blank line
// after the group goes too, since it only separated the constraint from the
// package clause.
func removal(file *token.File, d drop, src []byte) analysis.TextEdit {
	var (
		start, end = file.Offset(d.comment.Pos()), file.Offset(d.comment.End())
		lines      = 1
	)
	if d.alone {
		lines = 2
	}
	for range lines {
		if end < len(src) && src[end] == '\r' {
			end++
		}
		if end >= len(src) || src[end] != '\n' {
			break
		}
		end++
	}
	return analysis.TextEdit{Pos: file.Pos(start), End: file.Pos(end)}
}

// namedTags returns the tags the file name gives, and nil for a name that
// gives none. This copies go/build's rule for a name that matches *_GOOS,
// *_GOARCH or *_GOOS_GOARCH: the part after the first underscore decides, from
// its last one or two elements.
func namedTags(name string) []string {
	name, _, _ = strings.Cut(name, ".")
	i := strings.Index(name, "_")
	if i < 0 {
		return nil
	}
	parts := strings.Split(name[i:], "_")
	if n := len(parts); n > 0 && parts[n-1] == "test" {
		parts = parts[:n-1]
	}
	n := len(parts)
	switch {
	case n >= 2 && slices.Contains(allOS, parts[n-2]) &&
		slices.Contains(allArch, parts[n-1]):
		return []string{parts[n-2], parts[n-1]}
	case n >= 1 && (slices.Contains(allOS, parts[n-1]) ||
		slices.Contains(allArch, parts[n-1])):
		return []string{parts[n-1]}
	}
	return nil
}

// allOS and allArch are the values a file name can name, from the go command's
// own table of them, internal/syslist. A value added to the go command after
// this list is not recognized here: a name that ends in it then gives no tag,
// and the report waits until the value is added.
var (
	allOS = []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "hurd",
		"illumos", "ios", "js", "linux", "nacl", "netbsd", "openbsd",
		"plan9", "solaris", "wasip1", "windows", "zos",
	}
	allArch = []string{
		"386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be",
		"loong64", "mips", "mipsle", "mips64", "mips64le", "mips64p32",
		"mips64p32le", "ppc", "ppc64", "ppc64le", "riscv", "riscv64",
		"s390", "s390x", "sparc", "sparc64", "wasm",
	}
)
