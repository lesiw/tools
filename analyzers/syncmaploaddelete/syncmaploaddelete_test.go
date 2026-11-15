package syncmaploaddelete

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The fixture is a GOPATH tree, so its files have no version and the gate
// skips none of them. The width matches the one the commands set.
func TestAnalysisTest(t *testing.T) {
	linelen.Config.Len = 79
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// A module below Go 1.15 gets no diagnostics, and a file whose build
// constraint raises it gets them, with the fix.
func TestGoVersion(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.14")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "./go115")
}

// A renamed line that runs past the width the test sets is reported and left
// as written; a fix offered anyway would have no golden file to match. A line
// already past the width is fixed, since refusing cannot shorten it.
func TestWidth(t *testing.T) {
	linelen.Config.Len = 40
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "narrow")
}

// With no width set, nothing overflows, so every pair is fixed whatever its
// line length.
func TestNoWidth(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "nowidth")
}
