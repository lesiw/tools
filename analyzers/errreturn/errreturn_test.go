package errreturn

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
