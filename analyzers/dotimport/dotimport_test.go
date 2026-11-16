package dotimport

import (
	"embed"
	"flag"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	old := linelen.Config.Len
	linelen.Config.Len = 79
	defer func() { linelen.Config.Len = old }()
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer, "a",
		"mainpkg", "samealias", "samepkg", "sibling",
	)
}

func TestExempt(t *testing.T) {
	if !exempt("internal/types/errors") {
		t.Fatal("internal/types/errors is not exempt")
	}
	if err := addAllowed("example.com/constants"); err != nil {
		t.Fatal(err)
	}
	defer delete(allowed, "example.com/constants")
	if !exempt("example.com/constants") {
		t.Fatal("example.com/constants is not exempt")
	}
}

func TestAllowed(t *testing.T) {
	if err := addAllowed("example.com/constants"); err != nil {
		t.Fatal(err)
	}
	defer delete(allowed, "example.com/constants")
	dir := fixtures.Dir(t, testdata)
	analysistest.Run(t, dir, Analyzer, "allowed")
}

func TestFlags(t *testing.T) {
	fs := flag.NewFlagSet("dotimport", flag.ContinueOnError)
	Flags(fs)
	if err := fs.Parse([]string{"-allowed=example.com/other"}); err != nil {
		t.Fatal(err)
	}
	defer delete(allowed, "example.com/other")
	if !exempt("example.com/other") {
		t.Fatal("example.com/other is not exempt")
	}
}
