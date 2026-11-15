package wrapperfunc

import (
	"embed"
	"go/build"
	"go/parser"
	"go/token"
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

// TestConstrained covers the constraint forms a fixture cannot hold, since
// one would exclude the host, and the platform suffixes, which only name the
// machine running the test.
func TestConstrained(t *testing.T) {
	var (
		goos   = build.Default.GOOS
		goarch = build.Default.GOARCH
		cases  = []struct {
			test string
			name string
			src  string
			want bool
		}{{
			"no constraint",
			"path.go",
			"package p",
			false,
		}, {
			"go:build line",
			"path.go",
			"//go:build unix\n\npackage p",
			true,
		}, {
			"+build line",
			"path.go",
			"// +build unix\n\npackage p",
			true,
		}, {
			"prefix without a space",
			"path.go",
			"// +buildfoo\n\npackage p",
			false,
		}, {
			"name with no underscore",
			"linux.go",
			"package p",
			false,
		}, {
			"goos suffix",
			"path_" + goos + ".go",
			"package p",
			true,
		}, {
			"goarch suffix",
			"path_" + goarch + ".go",
			"package p",
			true,
		}, {
			"goos and goarch suffix",
			"path_" + goos + "_" + goarch + ".go",
			"package p",
			true,
		}}
	)
	for _, tt := range cases {
		t.Run(tt.test, func(t *testing.T) {
			file, err := parser.ParseFile(
				token.NewFileSet(), tt.name,
				tt.src, parser.ParseComments,
			)
			if err != nil {
				t.Fatal(err)
			}
			if got := constrained(tt.name, file); got != tt.want {
				t.Errorf(
					"constrained(%q) = %v, want %v",
					tt.name, got, tt.want,
				)
			}
		})
	}
}
