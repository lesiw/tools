package defaultcaseorder

import (
	"embed"
	"go/ast"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer, "a",
		"linedirective", "withheld",
	)
}

// TestStaleFile checks that a file which changed size after it was parsed gets
// no fix, since the offsets the move splices would point into other text. A
// fixture cannot hold the case: the size the analyzer sees and the bytes it
// reads agree in every tree a test can build.
func TestStaleFile(t *testing.T) {
	var (
		fset = token.NewFileSet()
		s    = &source{
			File: fset.AddFile("a.go", -1, 3),
			src:  []byte("package a\n"),
		}
		edits = s.move(new(ast.BlockStmt), clause{}, clause{}, clause{})
	)
	if edits != nil {
		t.Errorf("move on a stale file returned %d edits", len(edits))
	}
}
