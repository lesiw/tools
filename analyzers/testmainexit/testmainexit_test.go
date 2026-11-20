package testmainexit

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
	fixtures.Module(t, dir, "example.com/m", "1.14")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer,
		"example.com/m/aliased",
		"example.com/m/blank",
		"example.com/m/closure",
		"example.com/m/deferred",
		"example.com/m/deferredlit",
		"example.com/m/dotted",
		"example.com/m/exited",
		"example.com/m/helper",
		"example.com/m/imported",
		"example.com/m/kept",
		"example.com/m/norun",
		"example.com/m/returns",
		"example.com/m/shadowedos",
		"example.com/m/signature",
		"example.com/m/trailing",
		"example.com/m/tworuns",
	)
}

// From Go 1.15 the test wrapper passes the result of m.Run to os.Exit, so a
// TestMain that returns is correct.
func TestGoVersion(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.16")
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer,
		"example.com/m/current",
	)
}

// A file with no version of its own is checked against the whole language,
// where the wrapper exists.
func TestNoVersion(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "plain")
}
