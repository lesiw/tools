package strictvar

import (
	"embed"
	"go/ast"
	"go/token"
	"strings"
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
	var (
		dir     = fixtures.Dir(t, testdata)
		results = analysistest.RunWithSuggestedFixes(t, dir, Analyzer,
			"zero", "group", "block", "named", "makedecl",
			"preamble", "iife", "groupiife",
		)
	)
	checkNonZeroEdits(t, results)
}

// TestInlineBlock covers a shape gofmt erases, which is why it lives
// in testdata as .input: a one-line var block is reported as
// mergeable, and the fix is withheld because its spec shares the
// Lparen line.
func TestInlineBlock(t *testing.T) {
	analysistest.Run(t, fixtures.Dir(t, testdata), Analyzer, "inline")
}

// checkNonZeroEdits checks that the := fix ends where the value starts, so an
// edit inside the value cannot nest in it.
func checkNonZeroEdits(t *testing.T, results []*analysistest.Result) {
	t.Helper()
	for _, result := range results {
		pkg := result.Action.Package
		for _, d := range result.Action.Diagnostics {
			if !strings.HasPrefix(d.Message, nonzeroPrefix) {
				continue
			}
			at := pkg.Fset.Position(d.Pos)
			// A comment in the head withholds the fix, so a diagnostic
			// without one says nothing about where the edit ends.
			if len(d.SuggestedFixes) == 0 {
				continue
			}
			edits := d.SuggestedFixes[0].TextEdits
			if len(edits) != 1 {
				t.Errorf("got %d edits at %v, want 1", len(edits), at)
				continue
			}
			value := valueAt(pkg.Syntax, d.Pos)
			if value == nil {
				t.Errorf("no value at %v", at)
				continue
			}
			src := pkg.Fset.File(d.Pos)
			start := src.Offset(d.Pos)
			if got := src.Offset(edits[0].Pos); got != start {
				t.Errorf("edit at %v starts at %d, want %d", at, got, start)
			}
			got, want := src.Offset(edits[0].End), src.Offset(value.Pos())
			if got != want {
				t.Errorf("edit at %v ends at %d, want %d, where the value starts", at, got, want)
			}
		}
	}
}

// valueAt returns the value of the declaration that starts at pos.
func valueAt(files []*ast.File, pos token.Pos) ast.Expr {
	for _, file := range files {
		var value ast.Expr
		ast.Inspect(file, func(n ast.Node) bool {
			if value != nil {
				return false
			}
			gd, ok := n.(*ast.GenDecl)
			if !ok || gd.Pos() != pos || len(gd.Specs) == 0 {
				return true
			}
			vs, ok := gd.Specs[0].(*ast.ValueSpec)
			if ok && len(vs.Values) > 0 {
				value = vs.Values[0]
			}
			return false
		})
		if value != nil {
			return value
		}
	}
	return nil
}
