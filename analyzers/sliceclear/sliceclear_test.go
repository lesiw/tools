package sliceclear

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// testdata/src is loaded as a GOPATH tree, so a file in it has no version of
// its own and the gate skips nothing.
func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a", "shadowed")
}

// A module that declares a Go version below Go 1.21 gets no diagnostics,
// because the rewrite does not compile there. A file whose own build
// constraint raises it to Go 1.21 still gets them.
func TestGoVersion(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.20")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "example.com/m/go120")
}
