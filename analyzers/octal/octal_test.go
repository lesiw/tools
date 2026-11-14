package octal

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	// The copy holds no go.mod, so the type checker is given no version,
	// which is the empty-version default the guard treats as modern.
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestGoVersion pins where the guard falls: a module at Go 1.13, and a module
// below it holding a file that names no version beside files tagged go1.12 and
// go1.13. The test writes the module line itself, because //go:embed skips a
// directory that holds a go.mod.
func TestGoVersion(t *testing.T) {
	testVersion(t, "at", "1.13")
	testVersion(t, "below", "1.12")
}

func testVersion(t *testing.T, name, version string) {
	t.Helper()
	root := filepath.Join(fixtures.Dir(t, testdata), name)
	writeModule(t, root, version)
	analysistest.Run(t, root, Analyzer, "./a")
}

func writeModule(t *testing.T, root, version string) {
	t.Helper()
	err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module a\n\ngo "+version+"\n"),
		0o666,
	)
	if err != nil {
		t.Fatal(err)
	}
}
