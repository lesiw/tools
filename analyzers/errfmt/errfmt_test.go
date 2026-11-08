package errfmt

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalysisTest(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(),
		Analyzer, "a",
	)
}
