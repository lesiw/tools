package fix

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lesiw.io/tools/analyzers/linelen"
)

// update writes the golden files instead of comparing them.
var update = flag.Bool("update", false, "rewrite the case goldens")

// TestPipeline runs the whole pipeline over each case and compares the result
// with the golden beside it.
//
// A case is one file that builds, named .input so a formatter run over the
// repository leaves it alone. It is copied into a module of its own, settled
// by the same code the command runs, and compared byte for byte.
func TestPipeline(t *testing.T) {
	// The cases are what the tools produce, and a command sets the width
	// it runs with. The library default is no limit.
	linelen.Config.Len = 79
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	// The cases are read before any case changes directory, so every path
	// below is absolute.
	for i, input := range inputs {
		if inputs[i], err = filepath.Abs(input); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input")
		t.Run(name, func(t *testing.T) {
			work := copyCase(t, input)
			t.Chdir(work)
			if err := run([]string{"."}); err != nil {
				t.Fatalf("fix: %v", err)
			}
			compare(t, work, strings.TrimSuffix(input, ".input")+".golden")
		})
	}
}

// copyCase copies a case into a temporary module and returns it.
func copyCase(t *testing.T, input string) string {
	t.Helper()
	work := t.TempDir()
	src, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(work, "main.go")
	if err := os.WriteFile(out, src, 0o666); err != nil {
		t.Fatal(err)
	}
	gomod := "module case\n\ngo 1.26.0\n"
	mod := filepath.Join(work, "go.mod")
	if err := os.WriteFile(mod, []byte(gomod), 0o666); err != nil {
		t.Fatal(err)
	}
	return work
}

// compare checks the file the pipeline produced against its golden.
func compare(t *testing.T, work, golden string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(work, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(golden, got, 0o666); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
