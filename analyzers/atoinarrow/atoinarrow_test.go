package atoinarrow

import (
	"embed"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.Run(t, dir, Analyzer, "a")
}

func TestNarrowInt32(t *testing.T) {
	cases := []struct {
		arch string
		want bool
	}{
		{"386", false},
		{"amd64", true},
		{"wasm", true},
	}
	for _, tt := range cases {
		pass := &analysis.Pass{TypesSizes: types.SizesFor("gc", tt.arch)}
		if got := narrow(pass, types.Typ[types.Int32]); got != tt.want {
			t.Errorf(
				"narrow(int32) on %s = %v, want %v",
				tt.arch, got, tt.want,
			)
		}
	}
}
