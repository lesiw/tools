package elseif

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The commands run at this width, so the test does too.
func init() { linelen.Config.Len = 79 }

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestDirective holds the list to the comments a tool reads at a position, and
// to prose that opens like one.
func TestDirective(t *testing.T) {
	read := []string{
		"//line table.go:1",
		"/*line table.go:1*/",
		"//nolint",
		"//lint:ignore U1000",
		"//nosec",
	}
	for _, text := range read {
		if !directive(text) {
			t.Errorf("%q is not read as a directive", text)
		}
	}
	prose := []string{"// a note", "// Note: this", "//", "()"}
	for _, text := range prose {
		if directive(text) {
			t.Errorf("%q is read as a directive", text)
		}
	}
}
