package unusefultag

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
	analysistest.Run(t, dir, Analyzer, "a")
	analysistest.Run(t, dir, Analyzer, "encoding/json/fixture")
	analysistest.Run(t, dir, Analyzer, "encoding/json_test")
}

func TestSkipped(t *testing.T) {
	cases := map[string]bool{
		"encoding/json":         true,
		"encoding/json_test":    true,
		"encoding/json/fixture": true,
		"encoding/other":        false,
		"other":                 false,
	}
	for path, want := range cases {
		if got := skipped(path); got != want {
			t.Errorf("skipped(%q) = %v, want %v", path, got, want)
		}
	}
}
