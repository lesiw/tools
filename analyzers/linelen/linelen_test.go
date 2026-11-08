package linelen

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

// testdataDir materializes the embedded testdata into a fresh temp
// directory, restoring the .go extensions the fixtures are stored
// without. Fixtures live as .input in the repo so the formatter never
// touches them, but analysistest needs real .go files to parse. The
// returned dir holds the same src/ layout analysistest expects.
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
			// Drop the testdata/ prefix: the tree already carries src/.
			var (
				repoPath, _ = strings.CutPrefix(path, "testdata/")
				name, _     = strings.CutSuffix(repoPath, ".input")
				out         = filepath.Join(dir, name+".go")
			)
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

// The table is written as plain literals so a case reads as the line it
// describes, which puts it over the limit the table itself tests.
//
//ignore:linelen
var checkLineTests = []struct {
	name  string
	line  string
	allow bool
}{
	{"within the limit", `x := "short"`, true},
	{"string is continuous", `x := "the quick brown fox"`, true},
	{"string is not continuous", `x := "short" + abcdefgh`, false},
	{"several strings that each fall short", `x := "abc" + "def" + gh`, false},
	{"url is continuous", `// https://example.com`, true},
	{"url is not continuous", `// see https://e and more`, false},
	{"string and url each fall short", `x := "abc" + https://e`, false},
	{"url inside a string is counted once", `x := "ab://cd" + more`, false},
}

func TestCheckLine(t *testing.T) {
	c := Config{Len: 20}
	for _, tt := range checkLineTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.CheckLine([]byte(tt.line)); got != tt.allow {
				t.Errorf("CheckLine(%q) = %v, want %v", tt.line, got, tt.allow)
			}
		})
	}
}

func TestAnalysisTest(t *testing.T) {
	analysistest.Run(t, testdataDir(t), Analyzer,
		"basic", "longimportblock", "longimportstmt",
		"methodwrap", "continuous", "linedirective",
	)
}
