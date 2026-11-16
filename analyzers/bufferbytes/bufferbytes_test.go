package bufferbytes

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestSkipPackage(t *testing.T) {
	for _, path := range []string{"bytes", "bytes_test"} {
		if !skipPackage(path) {
			t.Errorf("skipPackage(%q) = false", path)
		}
	}
	if skipPackage("a") {
		t.Error("skipPackage(\"a\") = true")
	}
}
