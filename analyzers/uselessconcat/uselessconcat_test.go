package uselessconcat

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
)

// The commands set the width they run with; the tests run what the commands
// run.
func init() { linelen.Config.Len = 79 }

func TestAnalysisTest(t *testing.T) {
	dir := analysistest.TestData()
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}
