package builtinshadow

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
	analysistest.Run(t, dir, Analyzer, "a")
}

// A local declaration in a file below Go 1.18 gets no diagnostic for any or
// comparable, and one below Go 1.21 gets none for clear, max, or min. A
// package-level declaration is checked against every file in the package, so
// a raised file makes its name predeclared.
func TestGoVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		pkg     string
	}{
		{"Go117", "1.17", "example.com/m/v117"},
		{"Go117Pure", "1.17", "example.com/m/v117pure"},
		{"Go117Import", "1.17", "example.com/m/v117import"},
		{"Go118", "1.18", "example.com/m/v118"},
		{"Go120", "1.20", "example.com/m/v120"},
		{"Go121", "1.21", "example.com/m/v121"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := fixtures.Dir(t, testdata)
			fixtures.Module(t, dir, "example.com/m", test.version)
			analysistest.Run(t, dir, Analyzer, test.pkg)
		})
	}
}
