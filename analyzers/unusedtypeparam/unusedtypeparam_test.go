package unusedtypeparam

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
	fixtures.Module(t, dir, "example.com/a", "1.26")
	analysistest.Run(t, dir, Analyzer, "example.com/a/src/a")
}
