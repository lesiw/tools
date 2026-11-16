package sortstrings

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

// A module that declares a Go version below Go 1.21 gets no diagnostics for a
// file at that version, because slices.Sort does not exist there. A file whose
// build constraint raises it to Go 1.21 still gets them.
func TestGoVersion(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.20")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "example.com/m/go120")
}

// A package that slices imports cannot import slices, so its calls are
// reported without a fix.
func TestSlicesDependencies(t *testing.T) {
	paths := []string{"slices", "runtime", "cmp", "iter", "math/bits"}
	for _, path := range paths {
		if !slicesDep(path) {
			t.Errorf("%s: not read as a dependency of slices", path)
		}
	}
	if slicesDep("example.com/m") {
		t.Error("example.com/m: read as a dependency of slices")
	}
}
