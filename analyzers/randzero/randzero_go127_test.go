//go:build go1.27

package randzero

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

// TestMethodN checks math/rand/v2's generic Rand.N method, which Go 1.27
// introduced.
func TestMethodN(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.27")
	analysistest.Run(t, dir, Analyzer, "example.com/m/src/methodn")
}
