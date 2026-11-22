package mathrandseed

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// TestAtLeast holds the version test to the boundary and to the unknown case.
func TestAtLeast(t *testing.T) {
	var (
		file  = new(ast.File)
		cases = []struct {
			version string
			want    bool
		}{
			{"", false},
			{"go1.23", false},
			{"go1.24", true},
			{"1.24", true},
			{"go1.25", true},
		}
	)
	for _, tt := range cases {
		if got := atLeast(file, tt.version, "go1.24"); got != tt.want {
			t.Errorf("atLeast(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.24")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "example.com/m/src/a")
}

func TestPackageGodebug(t *testing.T) {
	var (
		fset = token.NewFileSet()
		src  = "package main\n"
	)
	plain, err := parser.ParseFile(fset, "a.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	marked, err := parser.ParseFile(
		fset,
		"b.go",
		"//go:debug randseednop=0\n\npackage main\n",
		parser.ParseComments,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !packageGodebug([]*ast.File{plain, marked}) {
		t.Fatal("packageGodebug missed a //go:debug line")
	}
}

func TestDeclaredHere(t *testing.T) {
	cases := []struct {
		owner, subject string
		want           bool
	}{
		{"math/rand", "math/rand", true},
		{"math/rand", "math/rand_test", true},
		{"math/rand", "math/rand_test [math/rand.test]", true},
		{"math/rand", "example.com/a", false},
	}
	for _, tt := range cases {
		var (
			pass = &analysis.Pass{Pkg: types.NewPackage(tt.subject, "p")}
			fn   = types.NewFunc(
				token.NoPos,
				types.NewPackage(tt.owner, "p"),
				"Seed",
				types.NewSignatureType(nil, nil, nil, nil, nil, false),
			)
		)
		if got := declaredHere(pass, fn); got != tt.want {
			t.Errorf(
				"declaredHere(%q, %q) = %v, want %v",
				tt.owner, tt.subject, got, tt.want,
			)
		}
	}
}

func TestFileGodebug(t *testing.T) {
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"x.go",
		"//go:debug randseednop=0\n\npackage main\n",
		parser.ParseComments,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !fileGodebug(file) {
		t.Fatal("a //go:debug line was not found")
	}
}

func TestDebugSetting(t *testing.T) {
	cases := []struct {
		text, want string
	}{
		{"//go:debug randseednop=0", "randseednop=0"},
		{"//go:debug\trandseednop=0", "randseednop=0"},
		{"// go:debug randseednop=0", ""},
		{"// unrelated", ""},
	}
	for _, tt := range cases {
		if got := debugSetting(tt.text); got != tt.want {
			t.Errorf("debugSetting(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
