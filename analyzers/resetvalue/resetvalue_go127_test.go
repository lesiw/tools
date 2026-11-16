//go:build go1.27

package resetvalue

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

// TestGoVersion127 checks the fix at Go 1.27, where the asynctimerchan setting
// has been removed.
func TestGoVersion127(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.27")
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "./at")
}
