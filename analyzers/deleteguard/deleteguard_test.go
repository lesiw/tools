package deleteguard

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The width matches the one the commands set.
func TestAnalysisTest(t *testing.T) {
	linelen.Config.Len = 79
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// The fix is offered for a line already past the width, because refusing
// cannot shorten it.
func TestOverlong(t *testing.T) {
	linelen.Config.Len = 40
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "narrow")
}

// A line past the width is reported without a fix when the guard's first line
// fits.
func TestWithhold(t *testing.T) {
	linelen.Config.Len = 20
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "withhold")
}

// The analyzer measures written lines, so a line directive does not change
// them.
func TestLineDirective(t *testing.T) {
	linelen.Config.Len = 79
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "linedirective")
}

// A chained rewrite past the width is reported without a fix when the first
// line of the else-if fits.
func TestWithholdChain(t *testing.T) {
	linelen.Config.Len = 30
	t.Cleanup(func() { linelen.Config.Len = 0 })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "withholdchain")
}
