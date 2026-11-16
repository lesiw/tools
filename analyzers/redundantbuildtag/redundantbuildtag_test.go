package redundantbuildtag

import (
	"cmp"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// TestConstraints checks what the rule reports, what it leaves alone, and the
// edit it offers. The cases drive the analyzer directly because analysistest
// cannot express them: a // want comment cannot sit on the line that is
// reported, because anything after the condition makes the constraint fail to
// parse and the package fail to load. A fixture named for another platform is
// not in the package either, so the file name and the module's Go version have
// to be inputs.
func TestConstraints(t *testing.T) {
	type testCase struct {
		name      string
		file      string // the name the build sees
		goVersion string // the module's language version
		src       string
		fix       string // the file after the fix, or "" when the line stands
	}
	cases := []testCase{{
		name: "tag the name gives",
		file: "a_windows.go",
		src:  "//go:build windows\n\npackage a\n",
		fix:  "package a\n",
	}, {
		name: "operating system and architecture the name gives",
		file: "a_linux_amd64.go",
		src:  "//go:build linux && amd64\n\npackage a\n",
		fix:  "package a\n",
	}, {
		name: "architecture the name gives",
		file: "a_arm64.go",
		src:  "//go:build arm64\n\npackage a\n",
		fix:  "package a\n",
	}, {
		name: "name carrying the test suffix",
		file: "a_linux_test.go",
		src:  "//go:build linux\n\npackage a\n",
		fix:  "package a\n",
	}, {
		name: "either of two tags the name settles",
		file: "a_linux.go",
		src:  "//go:build linux || amd64\n\npackage a\n",
		fix:  "package a\n",
	}, {
		name: "legacy lines beside the constraint",
		file: "a_linux_amd64.go",
		src: "//go:build linux && amd64\n// +build linux\n" +
			"// +build amd64\n\npackage a\n",
		fix: "package a\n",
	}, {
		name: "comment above the constraint",
		file: "a_windows.go",
		src:  "// Copyright 2026.\n\n//go:build windows\n\npackage a\n",
		fix:  "// Copyright 2026.\n\npackage a\n",
	}, {
		name: "legacy-looking line in package documentation",
		file: "a_windows.go",
		src: "// Package a reads items.\n" +
			"// +build windows and amd64 both hold here.\n\n" +
			"//go:build windows\n\npackage a\n",
		fix: "// Package a reads items.\n" +
			"// +build windows and amd64 both hold here.\n\n" +
			"package a\n",
	}, {
		name: "comment sharing the constraint's group",
		file: "a_windows.go",
		src:  "// Copyright 2026.\n//go:build windows\n\npackage a\n",
		fix:  "// Copyright 2026.\n\npackage a\n",
	}, {
		name: "line endings of the other kind",
		file: "a_windows.go",
		src:  "//go:build windows\r\n\r\npackage a\r\n",
		fix:  "package a\r\n",
	}, {
		name: "tag of another platform",
		file: "a_plan9.go",
		src:  "//go:build windows\n\npackage a\n",
	}, {
		name: "negation of the name's own tag",
		file: "a_windows.go",
		src:  "//go:build !windows\n\npackage a\n",
	}, {
		name: "negation of a tag the name does not give",
		file: "a_windows.go",
		src:  "//go:build !unix\n\npackage a\n",
	}, {
		name: "tag of the platform the name is paired with",
		file: "a_android.go",
		src:  "//go:build linux\n\npackage a\n",
	}, {
		name: "architecture the name leaves open",
		file: "a_linux.go",
		src:  "//go:build linux && amd64\n\npackage a\n",
	}, {
		name: "architecture the name does not give",
		file: "a_linux_arm64.go",
		src:  "//go:build linux && amd64\n\npackage a\n",
	}, {
		name: "operating system the name leaves open",
		file: "a.go",
		src:  "//go:build unix\n\npackage a\n",
	}, {
		name: "suffix that is not a platform",
		file: "a_unix.go",
		src:  "//go:build unix\n\npackage a\n",
	}, {
		name: "custom tag",
		file: "a.go",
		src:  "//go:build alpha\n\npackage a\n",
	}, {
		name: "tag the command line sets",
		file: "a.go",
		src:  "//go:build cgo\n\npackage a\n",
	}, {
		name: "condition over a tag the command line sets",
		file: "a.go",
		src:  "//go:build cgo || !cgo\n\npackage a\n",
	}, {
		name:      "release tag at the module's version",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.22\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tag go1.20 clamps to the module's go1.21",
		file:      "a.go",
		goVersion: "go1.21",
		src:       "//go:build go1.20\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tag beside one the file name gives",
		file:      "a_linux.go",
		goVersion: "go1.21",
		src:       "//go:build linux && go1.20\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tag on one side of an or",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.22 || cgo\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tags whose highest is the module's version",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.22 && go1.21\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tags whose highest is below the module's version",
		file:      "a.go",
		goVersion: "go1.23",
		src:       "//go:build go1.22 && go1.21\n\npackage a\n",
	}, {
		name:      "release tag on one side of an or in a later module",
		file:      "a.go",
		goVersion: "go1.23",
		src:       "//go:build go1.22 || cgo\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tag below the module on one side of an or",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.21 || cgo\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tags on both sides of an or below the module",
		file:      "a.go",
		goVersion: "go1.23",
		src:       "//go:build go1.22 || go1.23\n\npackage a\n",
	}, {
		name:      "module version without the Go prefix",
		file:      "a.go",
		goVersion: "1.22",
		src:       "//go:build go1.22\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "patch module version",
		file:      "a.go",
		goVersion: "go1.22.1",
		src:       "//go:build go1.22\n\npackage a\n",
		fix:       "package a\n",
	}, {
		name:      "release tag go1.20 clamps below the module's go1.22",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.20\n\npackage a\n",
	}, {
		name:      "release tag below the module's version",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.21\n\npackage a\n",
	}, {
		name:      "release tag above the module's version",
		file:      "a.go",
		goVersion: "go1.21",
		src:       "//go:build go1.22\n\npackage a\n",
	}, {
		name:      "module below Go 1.21",
		file:      "a.go",
		goVersion: "go1.20",
		src:       "//go:build go1.20\n\npackage a\n",
	}, {
		name:      "patch release tag",
		file:      "a.go",
		goVersion: "go1.22.1",
		src:       "//go:build go1.22.1\n\npackage a\n",
	}, {
		name:      "pre-release tag",
		file:      "a.go",
		goVersion: "go1.23",
		src:       "//go:build go1.21rc1\n\npackage a\n",
	}, {
		name:      "pre-release tag on one side of an or",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.22rc1 || cgo\n\npackage a\n",
	}, {
		name: "no module version",
		file: "a.go",
		src:  "//go:build go1.22\n\npackage a\n",
	}, {
		name:      "tag without a minor version",
		file:      "a.go",
		goVersion: "go1.23",
		src:       "//go:build go1\n\npackage a\n",
	}, {
		name:      "release tag before Go 1.1",
		file:      "a.go",
		goVersion: "go1.21",
		src:       "//go:build go1.0\n\npackage a\n",
	}, {
		name:      "release tag clamps below the module beside a name tag",
		file:      "a_linux.go",
		goVersion: "go1.22",
		src:       "//go:build linux && go1.21\n\npackage a\n",
	}, {
		name:      "release tag go1.20 clamps below the module beside a name tag",
		file:      "a_linux.go",
		goVersion: "go1.23",
		src:       "//go:build linux && go1.20\n\npackage a\n",
	}, {
		name:      "unsettled tag beside a release tag",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build go1.22 && cgo\n\npackage a\n",
	}, {
		name:      "negation of a release tag",
		file:      "a.go",
		goVersion: "go1.22",
		src:       "//go:build !go1.21\n\npackage a\n",
	}, {
		name: "legacy line beside a constraint that stands",
		file: "a_linux.go",
		src: "//go:build linux && amd64\n// +build linux\n\n" +
			"package a\n",
	}, {
		name: "legacy line without a constraint",
		file: "a_windows.go",
		src:  "// +build windows\n\npackage a\n",
	}, {
		name: "constraint after the package clause",
		file: "a_windows.go",
		src:  "package a\n\n//go:build windows\n",
	}, {
		name: "no constraint",
		file: "a.go",
		src:  "package a\n",
	}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fset, reported := analyze(t, tt.file, tt.goVersion, tt.src)
			if tt.fix == "" {
				for _, d := range reported {
					t.Errorf("reported %q, want nothing", d.Message)
				}
				return
			}
			if len(reported) != 1 {
				t.Fatalf("got %d diagnostics, want 1", len(reported))
			}
			d := reported[0]
			if d.Message != "redundant build constraint" {
				t.Errorf("message = %q", d.Message)
			}
			file := fset.File(d.Pos)
			at := tt.src[file.Offset(d.Pos):file.Offset(d.End)]
			if !strings.HasPrefix(at, "//go:build ") {
				t.Errorf("reported on %q, want the constraint", at)
			}
			fixes := d.SuggestedFixes
			if len(fixes) != 1 || len(fixes[0].TextEdits) == 0 {
				t.Fatalf("got %d fixes, want one with edits", len(fixes))
			}
			fixed := apply(tt.src, fset, fixes[0])
			if fixed != tt.fix {
				t.Errorf("fix =\n%q\nwant:\n%q", fixed, tt.fix)
			}
		})
	}
}

// analyze runs the rule over one file and returns what it reported, with the
// file set that locates it.
func analyze(t *testing.T, name, goVersion, src string) (fset *token.FileSet, reported []analysis.Diagnostic) {
	t.Helper()
	fset = token.NewFileSet()
	pass := &analysis.Pass{Analyzer: Analyzer, Fset: fset}
	if goVersion != "" {
		pass.Module = &analysis.Module{GoVersion: goVersion}
	}
	pass.ReadFile = func(string) ([]byte, error) { return []byte(src), nil }
	pass.Report = func(d analysis.Diagnostic) {
		reported = append(reported, d)
	}
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pass.Files = []*ast.File{file}
	if _, err := run(pass); err != nil {
		t.Fatal(err)
	}
	return fset, reported
}

// apply returns src with a fix's edits made, last one first so that each
// offset still addresses the bytes it was measured against.
func apply(src string, fset *token.FileSet, fix analysis.SuggestedFix) string {
	edits := slices.Clone(fix.TextEdits)
	if len(edits) == 0 {
		return src
	}
	slices.SortFunc(edits, func(x, y analysis.TextEdit) int {
		return cmp.Compare(y.Pos, x.Pos)
	})
	file := fset.File(edits[0].Pos)
	for _, edit := range edits {
		src = src[:file.Offset(edit.Pos)] + string(edit.NewText) +
			src[file.Offset(edit.End):]
	}
	return src
}
