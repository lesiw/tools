package earlyreturn

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
)

func TestAnalysisTest(t *testing.T) {
	linelen.Config.Len = 79
	t.Cleanup(func() { linelen.Config.Len = 0 })
	analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
}
