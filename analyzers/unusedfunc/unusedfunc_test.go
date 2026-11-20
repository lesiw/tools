package unusedfunc

import (
	"embed"
	"go/build"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestAdjacent checks deletions that meet. The fixes of one fixture are
// applied as one patch, so two suggested fixes that share a byte fail here
// rather than in a driver that applies one fix at a time.
func TestAdjacent(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "adjacent")
}

// TestToolchain checks that a package the Go toolchain owns is left alone.
// The fixture holds its packages under src/, the layout of a toolchain, so a
// GOROOT at the fixture's root makes one of them part of the standard library.
// GOROOT is process-wide, so the tests in this package stay serial.
func TestToolchain(t *testing.T) {
	var (
		dir   = fixtures.Dir(t, testdata)
		saved = build.Default.GOROOT
	)
	build.Default.GOROOT = dir
	t.Cleanup(func() { build.Default.GOROOT = saved })
	analysistest.Run(t, dir, Analyzer, "quiet", "quiet_test")
}
