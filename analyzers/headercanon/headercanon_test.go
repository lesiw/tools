package headercanon

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
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer, "a",
		"b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o",
	)
}

func TestFixesRemain(t *testing.T) {
	var (
		dir     = fixtures.Dir(t, testdata)
		results = analysistest.Run(t, dir, Analyzer, "a", "j")
		want    = []string{
			`header key "x-request-id" is not canonical`,
			`header key "x-canonical" is not canonical`,
			`header key "x-narrowed" is not canonical`,
			`header key "x-plain" is not canonical`,
		}
		found = make(map[string]struct{}, len(want))
	)
	for _, r := range results {
		for _, d := range r.Diagnostics {
			for _, msg := range want {
				if d.Message == msg && len(d.SuggestedFixes) > 0 {
					found[msg] = struct{}{}
				}
			}
		}
	}
	for _, msg := range want {
		if _, ok := found[msg]; !ok {
			t.Errorf("no fix for %s", msg)
		}
	}
}
