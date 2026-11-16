package packagename

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	analysistest.Run(t, fixtures.Dir(t, testdata), Analyzer,
		"allgenerated", "api", "common", "commons", "config",
		"generated", "helper", "helpers", "httputil", "interfaces",
		"linedirective", "misc", "miscellaneous", "mixedcaps", "onlytest",
		"pkg", "some_name", "testfile",
		"types", "util", "utilities", "utility", "utils",
	)
}
