package multichecker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lesiw.io/tools/internal/suite"
)

// TestWritePrunesImports checks that a fix which removes the last use of an
// import leaves a file that still builds. A round applies edits to source
// that has not been through the formatter, so the import has to be pruned
// afterwards or the next round cannot type-check the package.
func TestWritePrunesImports(t *testing.T) {
	var (
		src = `package p

import "sort"

func smallest(xs []int) int {
	sort.Ints(xs)
	return xs[0]
}
`
		name = filepath.Join(t.TempDir(), "p.go")
	)
	if err := os.WriteFile(name, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	// The span covers the call, as an analyzer replacing sort.Ints with
	// slices.Sort would produce.
	lo := strings.Index(src, "sort.Ints(xs)")
	hi := lo + len("sort.Ints(xs)")
	pending := []fixes{{
		name: "slicessort",
		files: map[string][]edit{
			name: {{lo: lo, hi: hi, text: []byte("slices.Sort(xs)")}},
		},
	}}
	if _, _, err := write(pending); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	if strings.Contains(out, `"sort"`) {
		t.Errorf("the unused sort import survived:\n%s", out)
	}
	if !strings.Contains(out, `"slices"`) {
		t.Errorf("the slices import was not added:\n%s", out)
	}
	if !strings.Contains(out, "slices.Sort(xs)") {
		t.Errorf("the edit was not applied:\n%s", out)
	}
}

// TestLoadReportsBuildErrors checks that a package which does not compile and
// which no fix repairs ends the run with an error: the last round still sees
// it, so there is no fixed point to report.
func TestLoadReportsBuildErrors(t *testing.T) {
	dir := t.TempDir()
	create(t, dir, "go.mod", goMod(t))
	create(t, dir, "a.go", "package p\n\nfunc f() { undefined() }\n")
	t.Chdir(dir)
	if err := run([]string{"."}, suite.Analyzers()); err == nil {
		t.Fatal("a package with a build error was reported as fixed")
	}
}

// TestFixEndsWhenTheErrorIsGone checks the other outcome: a tree whose only
// fault is the one a fix repairs. The round that repairs it still loads the
// package with its error, so the run has to end on what the last round saw, or
// a tree that builds again is reported as broken.
func TestFixEndsWhenTheErrorIsGone(t *testing.T) {
	dir := t.TempDir()
	create(t, dir, "go.mod", goMod(t))
	create(t, dir, "a.go", "package p\n\nfunc f() {\n\tv := 1\n}\n")
	t.Chdir(dir)
	if err := run([]string{"."}, suite.Analyzers()); err != nil {
		t.Fatalf("fix: %v", err)
	}
}
