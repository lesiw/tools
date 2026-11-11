package multichecker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// handshakes covers the argument shapes of a vet-tool run and a direct one.
var handshakes = []struct {
	name string
	args []string
	want bool
}{
	{"version handshake", []string{"-V=full"}, true},
	{"flag handshake", []string{"-flags"}, true},
	{"one package", []string{"-json", "/tmp/vet.cfg"}, true},
	{"a config alone", []string{"/tmp/x.cfg"}, true},
	{"direct run", []string{"./..."}, false},
	{"direct run with a flag", []string{"-printf=false", "."}, false},
}

// TestDrivenByGo covers the argument shapes the go command uses when it runs
// a binary as a vet tool.
func TestDrivenByGo(t *testing.T) {
	for _, tt := range handshakes {
		t.Run(tt.name, func(t *testing.T) {
			if got := drivenByGo(tt.args); got != tt.want {
				t.Errorf("drivenByGo(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// probe reports whether the package it was handed holds a test file.
var probe = &analysis.Analyzer{
	Name: "probe",
	Doc:  "reports whether the package holds a test file",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			name := pass.Fset.Position(f.Pos()).Filename
			if strings.HasSuffix(name, "_test.go") {
				pass.Reportf(f.Pos(), "package holds a test file")
			}
		}
		return nil, nil
	},
}

// plain and test are a package with one function and a test that calls it.
const (
	plain = "package p\n\nfunc F() int { return 1 }\n"
	test  = "package p\n\nimport \"testing\"\n\n" +
		"func TestF(t *testing.T) { _ = F() }\n"
)

// goMod returns the module file for a case, at the directive this module
// promises.
func goMod(t *testing.T) string {
	t.Helper()
	return "module example.com/p\n\ngo " + goDirective(t) + "\n"
}

// TestVetSeesTheTestVariant holds the command to the go command's choice: a
// package with tests is analyzed once, as its test variant, so the run sees
// the test files and no file is analyzed twice.
func TestVetSeesTheTestVariant(t *testing.T) {
	dir := t.TempDir()
	create(t, dir, "go.mod", goMod(t))
	create(t, dir, "a.go", plain)
	create(t, dir, "a_test.go", test)
	t.Chdir(dir)
	var runs int
	counted := *probe
	counted.Run = func(pass *analysis.Pass) (any, error) {
		runs++
		return probe.Run(pass)
	}
	code := vet([]*analysis.Analyzer{&counted}, []string{"."})
	if code != exitFinding {
		t.Fatalf("vet() = %d, want %d", code, exitFinding)
	}
	if runs != 1 {
		t.Errorf("analyzer ran %d times, want 1", runs)
	}
}

// TestVetWithoutTests checks the same package with no test file, where the
// package itself is the one to analyze.
func TestVetWithoutTests(t *testing.T) {
	dir := t.TempDir()
	create(t, dir, "go.mod", goMod(t))
	create(t, dir, "a.go", plain)
	t.Chdir(dir)
	code := vet([]*analysis.Analyzer{probe}, []string{"."})
	if code != exitOK {
		t.Fatalf("vet() = %d, want %d", code, exitOK)
	}
}

// A run that found something in one package and could not analyze another
// reports the failure, since a caller cannot act on findings from a run that
// did not finish.
func TestVetFailureOutranksFinding(t *testing.T) {
	var (
		dir = t.TempDir()
		bad = "package b\n\nfunc F() { undefined() }\n"
	)
	create(t, dir, "go.mod", goMod(t))
	create(t, dir, "good/a.go", plain)
	create(t, dir, "good/a_test.go", test)
	create(t, dir, "bad/b.go", bad)
	t.Chdir(dir)
	code := vet([]*analysis.Analyzer{probe}, []string{"./good", "./bad"})
	if code != exitFailed {
		t.Fatalf("vet() = %d, want %d", code, exitFailed)
	}
}

// create writes a file under dir.
func create(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
}

// help is a subcommand only as the first argument, so a flag ahead of it
// leaves a package pattern.
func TestHelpSubcommand(t *testing.T) {
	if !isHelp([]string{"help"}) {
		t.Fatal("isHelp(help) = false")
	}
	if isHelp([]string{"./..."}) {
		t.Fatal("isHelp(./...) = true")
	}
	if isHelp([]string{"-fix", "help"}) {
		t.Fatal("isHelp(-fix help) = true")
	}
}
