package mathrandseed

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	noWorkspace(t)
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.24")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "example.com/m/src/a")
}

// A tree with no module information is reported without a fix.
func TestFixableModule(t *testing.T) {
	cases := []struct {
		name   string
		module *analysis.Module
		want   bool
	}{
		{"nil", nil, false},
		{"non-main", &analysis.Module{GoVersion: "1.24"}, false},
		{"no version", &analysis.Module{Main: true}, false},
		{"below", &analysis.Module{Main: true, GoVersion: "1.23"}, false},
		{"from", &analysis.Module{Main: true, GoVersion: "1.24"}, true},
	}
	for _, tt := range cases {
		if got := fixableModule(tt.module); got != tt.want {
			t.Errorf("fixableModule(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestModuleGodebug(t *testing.T) {
	var (
		dir  = t.TempDir()
		name = filepath.Join(dir, "go.mod")
		src  = "module example.com/m\n\ngo 1.24\n\ngodebug randseednop=0\n"
	)
	if err := os.WriteFile(name, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	if !moduleGodebug(&analysis.Module{GoMod: name}) {
		t.Fatal("moduleGodebug missed randseednop=0")
	}
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

// noWorkspace turns off workspace mode for a test.
func noWorkspace(t *testing.T) {
	t.Helper()
	t.Setenv("GOWORK", "off")
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

func TestWorkspaceModuleFound(t *testing.T) {
	var (
		root = t.TempDir()
		dir  = filepath.Join(root, "m")
	)
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "go.work")
	if err := os.WriteFile(work, []byte("go 1.24\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "")
	if !workspace(&analysis.Module{Dir: dir}) {
		t.Fatal("go.work was not found above the module directory")
	}
}

func TestWorkspaceFound(t *testing.T) {
	var (
		dir  = t.TempDir()
		work = filepath.Join(dir, "go.work")
	)
	if err := os.WriteFile(work, []byte("go 1.24\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "")
	t.Chdir(dir)
	if !workspace(nil) {
		t.Fatal("go.work was not found from the working directory")
	}
}

func TestGodebug(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"godebug randseednop=0\n", true},
		{"godebug default=go1.23\n", true},
		{"godebug (\n\trandseednop=0\n)\n", true},
		{"godebug\trandseednop=0\n", true},
		{"godebug\t(\n\trandseednop=0\n)\n", true},
		{"godebug randseednop=1\n", false},
		{"godebug default=go1.24\n", false},
		{"// godebug randseednop=0\n", false},
	}
	for _, tt := range cases {
		if got := godebug([]byte(tt.src)); got != tt.want {
			t.Errorf("godebug(%q) = %v, want %v", tt.src, got, tt.want)
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
