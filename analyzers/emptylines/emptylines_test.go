package emptylines

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a", "linedirective")
}

// runCase is one shape of run, and the edit and message it draws.
type runCase struct {
	name        string
	fix         string
	first, last int // the lines the edit runs from and to
}

// TestRunFix checks the edit and the message of a run. A golden holds neither:
// analysistest never compares a fix message, and it formats both the fixed
// file and the golden, where gofmt collapses a run either way.
func TestRunFix(t *testing.T) {
	cases := []runCase{
		{"two", "Remove the extra blank line", 6, 7},
		{"three", "Remove the extra blank lines", 6, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package a\n\nfunc f() {\n\tprintln(1)\n\n\n"
			if tc.name == "three" {
				src += "\n"
			}
			src += "\tprintln(2)\n}\n"
			read := func(string) ([]byte, error) { return []byte(src), nil }
			var diags []analysis.Diagnostic
			report := func(d analysis.Diagnostic) { diags = append(diags, d) }
			fset := token.NewFileSet()
			syntax, err := parser.ParseFile(fset, "a.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			pass := &analysis.Pass{
				Fset:     fset,
				Files:    []*ast.File{syntax},
				ReadFile: read,
				Report:   report,
			}
			if _, err := Analyzer.Run(pass); err != nil {
				t.Fatal(err)
			}
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics, want 1", len(diags))
			}
			d := diags[0]
			if want := "more than one blank line in a row"; d.Message != want {
				t.Errorf("message is %q, want %q", d.Message, want)
			}
			if got := fset.Position(d.Pos).Line; got != 4 {
				t.Errorf("report is on line %d, want 4, over the run", got)
			}
			if len(d.SuggestedFixes) != 1 {
				t.Fatalf("got %d fixes, want 1", len(d.SuggestedFixes))
			}
			fix := d.SuggestedFixes[0]
			if fix.Message != tc.fix {
				t.Errorf("fix message is %q, want %q", fix.Message, tc.fix)
			}
			if len(fix.TextEdits) != 1 {
				t.Fatalf("got %d edits, want 1", len(fix.TextEdits))
			}
			first := fset.Position(fix.TextEdits[0].Pos).Line
			last := fset.Position(fix.TextEdits[0].End).Line
			if first != tc.first || last != tc.last {
				t.Errorf("edit runs from line %d to line %d, want %d to %d, which keeps the run's first line", first, last, tc.first, tc.last)
			}
		})
	}
}

// TestShortFile checks that a file shorter than the one that was parsed gets
// no report, since the lines the parse found do not index the bytes read.
func TestShortFile(t *testing.T) {
	const (
		src   = "package a\nfunc f() {\n\n\n\tprintln(1)\n}\n"
		short = "package a\nfunc f() {\n\n\n"
	)
	fset := token.NewFileSet()
	syntax, err := parser.ParseFile(fset, "a.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var diags []analysis.Diagnostic
	read := func(string) ([]byte, error) { return []byte(short), nil }
	report := func(d analysis.Diagnostic) { diags = append(diags, d) }
	pass := &analysis.Pass{
		Fset:     fset,
		Files:    []*ast.File{syntax},
		ReadFile: read,
		Report:   report,
	}
	if _, err := Analyzer.Run(pass); err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Errorf("got %d diagnostics on a stale file, want none", len(diags))
	}
}
