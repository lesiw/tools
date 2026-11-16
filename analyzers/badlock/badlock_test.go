package badlock

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The commands set the width they run with; the tests run what the commands
// run.
func init() { linelen.Config.Len = 79 }

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// The release is a longer name than the method it replaces, so the narrow
// fixture's deferred line crosses the limit once rewritten: the report stays,
// and the fixture has no golden file because no fix is suggested. Its want
// comment sits on the line above the deferred call, since a comment on that
// line would widen it and the fix would fit again.
func TestNarrowWidth(t *testing.T) {
	prev := linelen.Config.Len
	// The fixture's deferred line is 21 columns and the rewrite is 24, so the
	// limit is set to the line's own width to withhold the fix.
	linelen.Config.Len = 21
	t.Cleanup(func() { linelen.Config.Len = prev })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "narrow")
}
