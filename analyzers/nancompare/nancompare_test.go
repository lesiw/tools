package nancompare

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The commands set the width they run with; the tests run what the commands
// run.
func init() { linelen.Config.Len = 79 }

// Test checks which comparisons are reported and what the fix writes.
func Test(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestOneLine checks the guard that keeps a fix from joining lines.
func TestOneLine(t *testing.T) {
	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "a.go", "1 ==\n\t2", 0)
	if err != nil {
		t.Fatal(err)
	}
	bin := expr.(*ast.BinaryExpr)
	if oneLine(fset, bin.Pos(), bin.End()) {
		t.Fatal("oneLine accepted a comparison over two lines")
	}
}
