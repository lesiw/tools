// Package redundantbuildtag provides an analysis.Analyzer that reports a
// //go:build line the file name or the module's Go version already implies.
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
// The go command sets the release tags go1.1 through go1.N for the toolchain
// version 1.N, and a //go:build line can name one of them. Since Go 1.21 the
// go directive is a minimum version, so a module at Go 1.21 or later is built
// only by a toolchain that sets every release tag up to the module's version.
// A go1.X tag at or below that version holds in every build.
//
// Go also uses the //go:build line to set the file's language version, clamped
// to Go 1.21. The rule removes a line whose version settles the condition only
// when the line sets no version, so removing it changes nothing, or when its
// version after the clamp is the module's, so the file is already at the
// module's version. A line whose version after the clamp is not the module's
// is left alone, since dropping it would move the file to the module's
// version. A module below Go 1.21 is left alone too, because its go directive
// was advisory. A tag that is not one of the go1.N release tags is left alone
// as well, since the go command sets no such tag.
//
// Leaves alone: a file the current build does not include, since the go
// command does not offer it to the analyzer and a run on the platform the name
// selects reports it. A // +build line in a file that is not reported for
// another reason is left alone too, since the suite's plusbuild pass reports
// that line in a file at Go 1.18 or later and both analyzers would then report
// the same line. A // +build line in a group with another comment is left
// alone, since it may be documentation; gofmt writes the //go:build line back
// from it, and the next run has the same line to report. The rule also leaves
// a file whose only constraint is a // +build line, since gofmt writes the
// //go:build line from it and the next run has something to report.
//
// The fix removes the line, and the blank line after it when the line is the
// whole of its comment group. A // +build line in a group that holds only
// build constraints goes too: the build ignores those while a //go:build line
// is present, and gofmt would write the //go:build line back from one, so the
// redundancy would return.
//
// The rule is adapted from revive's redundant-build-tag.
package redundantbuildtag

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/token"
	"go/version"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "redundantbuildtag",
	Doc:  "reports a build constraint the file name or module version implies",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	goVersion := moduleVersion(pass)
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		if err := check(pass, file, name, goVersion); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// moduleVersion returns the language version the package's module declares.
func moduleVersion(pass *analysis.Pass) string {
	if pass.Module == nil {
		return ""
	}
	return validVersion(pass.Module.GoVersion)
}

// validVersion returns v as a Go version, adding the prefix the go command
// leaves off a module's version.
func validVersion(v string) string {
	if version.IsValid(v) {
		return v
	}
	if v := "go" + v; version.IsValid(v) {
		return v
	}
	return ""
}

// check reports the file's build constraint when the name or the module's Go
// version already gives it.
func check(pass *analysis.Pass, file *ast.File, name, goVersion string) error {
	comment, expr, drops := constraintLines(file)
	if expr == nil {
		return nil
	}
	named := namedTags(filepath.Base(name))
	settles := func(tag string) bool {
		return slices.Contains(named, tag) || release(tag, goVersion)
	}
	if !always(expr, settles) || !keepsVersion(expr, goVersion) {
		return nil
	}
	return report(pass, comment, drops)
}

// release reports whether every build the module allows sets tag. The module's
// go directive was advisory before Go 1.21, so an older module guarantees
// nothing.
func release(tag, goVersion string) bool {
	if !version.IsValid(goVersion) {
		return false
	}
	if version.Compare(goVersion, "go1.21") < 0 {
		return false
	}
	if !releaseTag(tag) {
		return false
	}
	return version.Compare(goVersion, tag) >= 0
}

// releaseTag reports whether tag is one of the go1.N tags the go command sets.
func releaseTag(tag string) bool {
	if !version.IsValid(tag) {
		return false
	}
	if version.Compare(tag, "go1.1") < 0 {
		return false
	}
	return version.Lang(tag) == tag
}

// keepsVersion reports whether removing the line leaves the file's language
// version alone. A line that names no version leaves the version to the
// module; one that names a version has it clamped to Go 1.21 before the
// comparison.
func keepsVersion(expr constraint.Expr, goVersion string) bool {
	v := constraint.GoVersion(expr)
	if v == "" {
		return true
	}
	if !version.IsValid(goVersion) {
		return false
	}
	if version.Compare(v, "go1.21") < 0 {
		v = "go1.21"
	}
	return version.Lang(v) == version.Lang(goVersion)
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
			case constraint.IsPlusBuild(c.Text) && alone:
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
// file. A tag the name and the module's Go version do not settle may be set or
// unset, so an expression holding one can go either way and the answer is no.
func always(expr constraint.Expr, holds func(string) bool) bool {
	switch expr := expr.(type) {
	case *constraint.TagExpr:
		return holds(expr.Tag)
	case *constraint.AndExpr:
		return always(expr.X, holds) && always(expr.Y, holds)
	case *constraint.OrExpr:
		return always(expr.X, holds) || always(expr.Y, holds)
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
