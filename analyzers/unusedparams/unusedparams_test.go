package unusedparams

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// Test checks which parameters are reported and what the fix writes.
func Test(t *testing.T) {
	analysistest.RunWithSuggestedFixes(
		t, fixtures.Dir(t, testdata),
		Analyzer, "a",
	)
}
