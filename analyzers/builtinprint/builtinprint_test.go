package builtinprint

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
	dir := module(t, fixtures.Dir(t, testdata), "example.com/m")
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer,
		"example.com/m/a", "example.com/m/declared",
	)
}

func TestNoModule(t *testing.T) {
	dir := filepath.Join(fixtures.Dir(t, testdata), "gopath")
	analysistest.Run(t, dir, Analyzer, "nomodule")
}

// module writes a go.mod for dir and returns dir.
func module(t *testing.T, dir, path string) string {
	t.Helper()
	var (
		gomod    = filepath.Join(dir, "go.mod")
		contents = "module " + path + "\n\ngo 1.21\n"
	)
	if err := os.WriteFile(gomod, []byte(contents), 0o666); err != nil {
		t.Fatal(err)
	}
	return dir
}
