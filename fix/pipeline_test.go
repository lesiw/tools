package fix

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"lesiw.io/tools/analyzers/linelen"
)

// update writes the golden files instead of comparing them.
var update = flag.Bool("update", false, "rewrite the case goldens")

// pipelineCase is one case: its name, the file it starts from, and the golden
// it has to end at.
type pipelineCase struct {
	name   string
	input  string
	golden string
}

// TestPipeline runs the whole pipeline over every case and compares each
// result with the golden beside it.
//
// A case is one file that builds, named .input so a formatter run over the
// repository leaves it alone. Every case goes into one module, one package
// per case, so the whole corpus is settled by one load. A case that does not
// settle stops neither the load nor the other cases' comparisons.
func TestPipeline(t *testing.T) {
	// The cases are what the tools produce, and a command sets the width
	// it runs with. The library default is no limit.
	linelen.Config.Len = 79
	var (
		cases = readCases(t)
		work  = buildModule(t, cases)
	)
	// Chdir is process-wide, so this test stays serial.
	t.Chdir(work)
	if err := run([]string{"./..."}); err != nil {
		// This is setup for every case, so the comparisons still run.
		t.Errorf("fix ./...: %v", err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { compare(t, work, c) })
	}
}

// readCases returns the cases, in the order their files sort.
func readCases(t *testing.T) (cases []pipelineCase) {
	t.Helper()
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range inputs {
		if input, err = filepath.Abs(input); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, pipelineCase{
			name:   strings.TrimSuffix(filepath.Base(input), ".input"),
			input:  input,
			golden: strings.TrimSuffix(input, ".input") + ".golden",
		})
	}
	return cases
}

// buildModule copies every case into one temporary module, one package per
// case, and returns the module's directory.
func buildModule(t *testing.T, cases []pipelineCase) string {
	t.Helper()
	var (
		work      = t.TempDir()
		gomod     = filepath.Join(work, "go.mod")
		directive = "module cases\n\ngo " + goDirective(t) + "\n"
	)
	if err := os.WriteFile(gomod, []byte(directive), 0o666); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		src, err := os.ReadFile(c.input)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(work, c.name)
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "main.go")
		if err := os.WriteFile(out, src, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	return work
}

// compare checks the file the pipeline produced against its golden.
func compare(t *testing.T, work string, c pipelineCase) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(work, c.name, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(c.golden, got, 0o666); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(c.golden)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Errorf("fix ./... (-want +got):\n%s", diff)
	}
}

// goDirective returns the go directive for the generated module. It reads the
// directive this module promises, so a case cannot use syntax that a supported
// toolchain rejects.
func goDirective(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal("could not read go.mod:", err)
	}
	for line := range strings.Lines(string(data)) {
		if v, ok := strings.CutPrefix(line, "go "); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Fatal("../go.mod has no go directive")
	return ""
}
