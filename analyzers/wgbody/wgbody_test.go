package wgbody

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
	"lesiw.io/tools/internal/ignore"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.Run(t, dir, Analyzer, "a")
}

func TestIgnore(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.Run(t, dir, ignore.Apply(Analyzer)[0], "ignored")
}
