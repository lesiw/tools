package tidytypes

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalysisTest(t *testing.T) {
	var (
		testdata = analysistest.TestData()
		pkgs     = []string{"basic", "parameters", "returns", "variadic"}
	)
	for _, pkg := range pkgs {
		t.Run(pkg, func(t *testing.T) {
			analysistest.Run(t, testdata, Analyzer, pkg)
		})
	}
	for _, dir := range []string{"fix"} {
		t.Run(dir, func(t *testing.T) {
			analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, dir)
		})
	}
}
