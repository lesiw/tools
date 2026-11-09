package bookends

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

// The commands set the width they run with; the tests run what the commands
// run.
func init() { linelen.Config.Len = 79 }

//go:embed testdata
var testdata embed.FS

// Test checks which pairs are reported and what the fix writes.
//
// The harness formats both the golden and the fixed bytes before comparing
// them, so the indent a fix writes is checked by the printer rather than by
// the comparison.
func Test(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}
