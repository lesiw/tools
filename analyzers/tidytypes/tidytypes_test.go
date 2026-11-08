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
	t.Run("fix", func(t *testing.T) {
		analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, "fix")
	})
}
