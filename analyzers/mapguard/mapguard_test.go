package mapguard

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
	previous := linelen.Config.Len
	linelen.Config.Len = 79
	t.Cleanup(func() { linelen.Config.Len = previous })
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// A fix that writes past the width is withheld while the guard's first line
// fits, offered once that line is past it too, and offered when no width is
// set.
func TestWidth(t *testing.T) {
	tests := []struct {
		name  string
		width int
		pkg   string
	}{
		{name: "write past the width", width: 45, pkg: "narrow"},
		{name: "guard past the width", width: 30, pkg: "past"},
		{name: "no width set", width: 0, pkg: "nowidth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := linelen.Config.Len
			linelen.Config.Len = tt.width
			t.Cleanup(func() { linelen.Config.Len = previous })
			dir := fixtures.Dir(t, testdata)
			analysistest.RunWithSuggestedFixes(t, dir, Analyzer, tt.pkg)
		})
	}
}
