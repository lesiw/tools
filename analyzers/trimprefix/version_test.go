package trimprefix

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

// TestVersionGate checks the language version and flag that decide whether a
// file is reported.
func TestVersionGate(t *testing.T) {
	cases := []struct {
		name    string
		version string
		modern  bool
		want    string
	}{{
		"from Go 1.20",
		"1.20",
		false,
		"",
	}, {
		"from Go 1.20 with the flag",
		"1.20",
		true,
		" // want `HasPrefix guard around TrimPrefix`",
	}, {
		"before Go 1.20",
		"1.19",
		false,
		" // want `HasPrefix guard around TrimPrefix`",
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modern = tc.modern
			t.Cleanup(func() { modern = false })
			dir := module(t, tc.version, tc.want)
			analysistest.Run(t, dir, Analyzer, "a")
		})
	}
}

// TestBeforeCutPrefix checks the version test, including the empty version a
// loader can supply.
func TestBeforeCutPrefix(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    bool
	}{
		{"empty", "", false},
		{"before", "go1.19", true},
		{"at", "go1.20", false},
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
			if got := beforeCutPrefix(pass, file); got != tc.want {
				t.Errorf(
					"beforeCutPrefix(%q) = %v, want %v",
					tc.version, got, tc.want,
				)
			}
		})
	}
}

// module writes a one-file module at the given language version, with want
// appended to the if line.
func module(t *testing.T, goversion, want string) string {
	t.Helper()
	var (
		dir  = t.TempDir()
		file = filepath.Join(dir, "a.go")
	)
	fixtures.Module(t, dir, "a", goversion)
	src := fmt.Sprintf(`package a

import "strings"

func f(s, prefix string) string {
	if strings.HasPrefix(s, prefix) {%s
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}
`, want,
	)
	if err := os.WriteFile(file, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	return dir
}
