package builtinprint

import (
	"embed"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.21")
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer,
		"example.com/m/a", "example.com/m/declared",
	)
}

func TestNoModule(t *testing.T) {
	dir := filepath.Join(fixtures.Dir(t, testdata), "gopath")
	analysistest.Run(t, dir, Analyzer, "nomodule")
}
