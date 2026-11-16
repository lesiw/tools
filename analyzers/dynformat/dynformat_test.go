package dynformat

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The fix covers the whole call. The printf pass inserts "%s", and the space
// after it before the format operand of the same call, and a selector-only
// edit would combine with that insertion to write fmt.Print("%s", s).
func TestAnalysisTest(t *testing.T) {
	var (
		dir     = fixtures.Dir(t, testdata)
		results = analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
	)
	for _, result := range results {
		for _, d := range result.Diagnostics {
			if len(d.SuggestedFixes) == 0 {
				t.Errorf("diagnostic at %v has no suggested fix", d.Pos)
			}
			for _, fix := range d.SuggestedFixes {
				for _, edit := range fix.TextEdits {
					if edit.Pos != d.Pos || edit.End != d.End {
						t.Errorf(
							"fix %q covers %v..%v, want the call at %v..%v",
							fix.Message, edit.Pos, edit.End, d.Pos, d.End,
						)
					}
				}
			}
		}
	}
}
