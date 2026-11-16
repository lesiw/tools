package trimprefix

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
	fixtures.Module(t, dir, "a", "1.19")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a/src/a")
}
