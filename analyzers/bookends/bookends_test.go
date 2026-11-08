package bookends

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

//go:embed testdata
var testdata embed.FS

// testdataDir copies the embedded tree into a temporary directory, restoring
// the .go extension the harness expects: .input becomes .go and
// .input.golden becomes .go.golden. The .input form keeps the fixtures away
// from a formatter run over the repository.
func testdataDir(t *testing.T) string {
	t.Helper()
	var (
		dir  = t.TempDir()
		walk = func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			content, err := testdata.ReadFile(path)
			if err != nil {
				return err
			}
			var (
				repoPath, _ = strings.CutPrefix(path, "testdata/")
				name, ok    = strings.CutSuffix(repoPath, ".input.golden")
			)
			if ok {
				name += ".go.golden"
			} else {
				name, _ = strings.CutSuffix(repoPath, ".input")
				name += ".go"
			}
			out := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(out), 0777); err != nil {
				return err
			}
			return os.WriteFile(out, content, 0666)
		}
	)
	if err := fs.WalkDir(testdata, ".", walk); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Test checks which pairs are reported and what the fix writes.
//
// The harness formats both the golden and the fixed bytes before comparing
// them, so the indent a fix writes is checked by the printer rather than by
// the comparison.
func Test(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, testdataDir(t), Analyzer, "a")
}
