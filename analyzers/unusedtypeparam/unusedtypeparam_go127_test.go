//go:build go1.27

package unusedtypeparam

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

// TestGenericMethods checks type parameters declared on methods, which Go 1.27
// introduced.
func TestGenericMethods(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	fixtures.Module(t, dir, "example.com/m", "1.27")
	analysistest.Run(t, dir, Analyzer, "example.com/m/src/genericmethod")
}
