package rangevaladdress

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// testdata/src is a GOPATH tree, so its files carry no version and the loops
// that declare their variables go unreported.
func TestUnversioned(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.Run(t, dir, Analyzer, "a")
}

// TestAnalysisTest runs the shapes in a module that declares go 1.21, where
// every iteration shares the loop variable, and checks the copy each report
// suggests.
func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.21")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "example.com/m/go121")
}

func TestGo122(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.22")
	analysistest.Run(t, dir, Analyzer, "example.com/m/go122")
}
