package bidicontrol

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
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestDirective(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"//line alpha.go:1", true},
		{"/*line alpha.go:1*/", true},
		{"//extern alpha", true},
		{"//export alpha", true},
		{"//go:generate alpha", true},
		{"//nolint", true},
		{"//sys alpha", true},
		{"//sysnb alpha", true},
		{"// +build alpha", true},
		{"//+build alpha", true},
		{"//  +build alpha", true},
		{"//\t+build alpha", true},
		{"// Output: alpha", true},
		{"// Unordered output: alpha", true},
		{"// alpha", false},
		{"/* alpha */", false},
	}
	for _, tc := range cases {
		if got := directive(tc.text); got != tc.want {
			t.Errorf("directive(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
