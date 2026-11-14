package boolreturn

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	linelen.Config.Len = 79
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestNarrowWidth checks the width guard on its own terms: at ten columns no
// return fits, so every pair the fixture holds is reported and none is fixed.
func TestNarrowWidth(t *testing.T) {
	linelen.Config.Len = 10
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}
