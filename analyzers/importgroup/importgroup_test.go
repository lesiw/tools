package importgroup

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// TestIndent checks that a spec's block comment keeps its interior, which a
// second round would otherwise rewrite again.
func TestIndent(t *testing.T) {
	var (
		in   = "/*\n\ta note\n*/\n\"fmt\""
		want = "\t/*\n\ta note\n*/\n\"fmt\""
	)
	if got := indent(in); got != want {
		t.Errorf("indent(%q) = %q, want %q", in, got, want)
	}
	if got := indent("\"os\""); got != "\t\"os\"" {
		t.Errorf("indent(\"os\") = %q, want %q", got, "\t\"os\"")
	}
}

// Test checks which blocks are reported and what the fix writes.
func Test(t *testing.T) {
	if err := Analyzer.Flags.Set("local", "lesiw.io"); err != nil {
		t.Fatal(err)
	}
	analysistest.RunWithSuggestedFixes(
		t, fixtures.Dir(t, testdata),
		Analyzer, "a", "b", "c", "d", "e", "f", "g",
	)
}
