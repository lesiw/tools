package resetvalue

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// TestAnalysisTest leaves the tree with no module, so every file names no
// version and the fix is offered.
func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestGoVersion pins the fix boundary. Below Go 1.23 the repair is the
// Stop-and-drain sequence, and Go 1.23 through 1.26 can restore the buffered
// channel with asynctimerchan, so both report without a fix. Go 1.27 removed
// the setting and gets the deletion.
func TestGoVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		pkg     string
	}{
		{"below", "1.22", "./below"},
		{"setting", "1.26", "./setting"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := fixtures.Dir(t, testdata)
			fixtures.Module(t, dir, "example.com/m", tt.version)
			analysistest.RunWithSuggestedFixes(t, dir, Analyzer, tt.pkg)
		})
	}
}
