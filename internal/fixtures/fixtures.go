// Package fixtures holds the harness code the analyzer tests share.
//
// A fixture is stored as .input so a run of the tools over this repository
// leaves it alone: the tools only see .go files. The harness needs real .go
// files to parse, so the tree is copied into a temporary directory with the
// extension restored.
package fixtures

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Dir copies an embedded testdata tree into a temporary directory and returns
// it. The testdata/ prefix is dropped, .input becomes .go, and .input.golden
// becomes .go.golden.
func Dir(t *testing.T, testdata fs.FS) string {
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
			content, err := fs.ReadFile(testdata, path)
			if err != nil {
				return err
			}
			repoPath, _ := strings.CutPrefix(path, "testdata/")
			name, ok := strings.CutSuffix(repoPath, ".input.golden")
			if ok {
				name += ".go.golden"
			} else {
				name, _ = strings.CutSuffix(repoPath, ".input")
				name += ".go"
			}
			out := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
				return err
			}
			return os.WriteFile(out, content, 0o666)
		}
	)
	if err := fs.WalkDir(testdata, ".", walk); err != nil {
		t.Fatal(err)
	}
	return dir
}
