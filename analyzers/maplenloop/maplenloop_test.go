package maplenloop

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
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestWidth(t *testing.T) {
	old := linelen.Config.Len
	linelen.Config.Len = 48
	t.Cleanup(func() { linelen.Config.Len = old })
	dir := fixtures.Dir(t, testdata)
	results := analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "width")
	var fixes int
	for _, result := range results {
		for _, d := range result.Diagnostics {
			if len(d.SuggestedFixes) > 0 {
				fixes++
			}
		}
	}
	if fixes != 2 {
		t.Fatalf("fixes = %d, want 2", fixes)
	}
}
