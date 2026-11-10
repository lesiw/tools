package offby1

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalysisTest(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
}
