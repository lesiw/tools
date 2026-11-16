package blankimports

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
	analysistest.Run(t, dir, Analyzer, "a", "mainpkg")
}

func TestExempt(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		embedded bool
		want     bool
	}{{
		name: "unsafe",
		path: "unsafe",
		want: true,
	}, {
		name:     "embed",
		path:     "embed",
		embedded: true,
		want:     true,
	}, {
		name: "embed without directive",
		path: "embed",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exempt(tt.path, tt.embedded); got != tt.want {
				t.Fatalf("exempt = %v, want %v", got, tt.want)
			}
		})
	}
}
