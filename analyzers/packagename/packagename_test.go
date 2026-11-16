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
		"pkg", "some_name", "stdcollision/http",
		"stdcollision/internal/http", "stdcollision/json",
		"stdcollision/url", "stdcollision/vendor/http", "testfile",
		"types", "util", "utilities", "utility", "utils",
	)
}

func TestStandardCollision(t *testing.T) {
	cases := []struct {
		caseName   string
		name       string
		importPath string
		want       string
	}{{
		caseName:   "collision",
		name:       "http",
		importPath: "example.com/dir/http",
		want:       "net/http",
	}, {
		caseName:   "standard library package",
		name:       "http",
		importPath: "net/http",
	}, {
		caseName:   "standard library module path",
		name:       "http",
		importPath: "std/net/http",
	}, {
		caseName:   "standard library module base",
		name:       "bufio",
		importPath: "std/bufio",
	}, {
		caseName:   "standard library module other package",
		name:       "http",
		importPath: "std/http",
		want:       "net/http",
	}, {
		caseName:   "standard library internal",
		name:       "http",
		importPath: "std/internal/http",
	}, {
		caseName:   "internal package",
		name:       "http",
		importPath: "example.com/internal/http",
	}, {
		caseName:   "internal segment",
		name:       "http",
		importPath: "example.com/internal/http/client",
	}, {
		caseName:   "internal root",
		name:       "http",
		importPath: "internal/http",
	}, {
		caseName:   "vendored package",
		name:       "http",
		importPath: "example.com/vendor/http",
	}, {
		caseName:   "vendor segment",
		name:       "http",
		importPath: "example.com/vendor/http/client",
	}, {
		caseName:   "no collision",
		name:       "httputil",
		importPath: "example.com/dir/httputil",
	}}
	for _, tt := range cases {
		t.Run(tt.caseName, func(t *testing.T) {
			got := standardCollision(tt.name, tt.importPath)
			if got != tt.want {
				t.Errorf(
					"standardCollision(%q, %q) = %q, want %q",
					tt.name, tt.importPath, got, tt.want,
				)
			}
		})
	}
}
