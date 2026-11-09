package strictvar

import (
	"embed"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestSingleUseStrings(t *testing.T) {
	analysistest.Run(t, fixtures.Dir(t, testdata), Analyzer, "singleuse")
}

func TestSuggestedFixes(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, fixtures.Dir(t, testdata), Analyzer,
		"zero", "group", "block", "named", "makedecl",
		"preamble", "iife", "groupiife",
	)
}

// TestInlineBlock covers a shape gofmt erases, which is why it lives
// in testdata as .input: a one-line var block is reported as
// mergeable, and the fix is withheld because its spec shares the
// Lparen line.
func TestInlineBlock(t *testing.T) {
	analysistest.Run(t, fixtures.Dir(t, testdata), Analyzer, "inline")
}
