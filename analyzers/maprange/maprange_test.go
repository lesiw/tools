package maprange

import (
	"embed"
	"go/ast"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// src is a GOPATH tree, so the standard library's maps is the package the
// fixture names. dotted has no golden: a call named through a dot import is
// reported without a fix.
func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a", "dotted")
}

// The x/exp half of the rule matches on the import path, so its fixtures are
// modules whose path is golang.org/x/exp and whose maps package returns
// slices. That package is a pattern too, which gives the skip something to
// skip.
func TestGoVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		pkg     string
	}{
		{"Go 1.21", "1.21", "go121"},
		{"Go 1.24", "1.24", "go124"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := fixtures.Dir(t, testdata)
			fixtures.Module(t, dir, "golang.org/x/exp", tc.version)
			analysistest.RunWithSuggestedFixes(t, dir, Analyzer,
				"golang.org/x/exp/maps",
				"golang.org/x/exp/"+tc.pkg,
			)
		})
	}
}

// TestRangesInt pins what each file version means for the len(m) fix,
// including the empty version a loader can supply.
func TestRangesInt(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    bool
	}{
		{"no version", "", true},
		{"before Go 1.22", "go1.21", false},
		{"from Go 1.22", "go1.22", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				file = new(ast.File)
				pass = &analysis.Pass{
					TypesInfo: &types.Info{
						FileVersions: map[*ast.File]string{file: tc.version},
					},
				}
			)
			if got := rangesInt(pass, file); got != tc.want {
				t.Errorf(
					"rangesInt(%q) = %v, want %v",
					tc.version, got, tc.want,
				)
			}
		})
	}
}
