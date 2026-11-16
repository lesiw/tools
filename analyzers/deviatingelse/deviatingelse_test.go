package deviatingelse

import (
	"bytes"
	"cmp"
	"embed"
	"go/format"
	"go/token"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestWithheldCasesHaveNoFix(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	for _, result := range analysistest.Run(t, dir, Analyzer, "b") {
		for _, diag := range result.Diagnostics {
			if len(diag.SuggestedFixes) > 0 {
				t.Errorf(
					"%s: withheld case got a fix",
					result.Pass.Fset.Position(diag.Pos),
				)
			}
		}
	}
}

func TestSuggestedFixesAreFormatted(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	for _, result := range analysistest.Run(t, dir, Analyzer, "a") {
		for _, diag := range result.Diagnostics {
			for _, fix := range diag.SuggestedFixes {
				checkFormatted(t, result.Pass, fix)
			}
		}
	}
}

// checkFormatted applies fix's edits and reports whether gofmt would change
// the result.
func checkFormatted(t *testing.T, pass *analysis.Pass, fix analysis.SuggestedFix) {
	t.Helper()
	files := make(map[*token.File][]analysis.TextEdit)
	for _, edit := range fix.TextEdits {
		file := pass.Fset.File(edit.Pos)
		files[file] = append(files[file], edit)
	}
	for file, edits := range files {
		src, err := pass.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		slices.SortFunc(edits, func(x, y analysis.TextEdit) int {
			return cmp.Compare(y.Pos, x.Pos)
		})
		out := bytes.Clone(src)
		for _, edit := range edits {
			lo, hi := file.Offset(edit.Pos), file.Offset(edit.End)
			out = append(
				append(append([]byte{}, out[:lo]...), edit.NewText...),
				out[hi:]...,
			)
		}
		formatted, err := format.Source(out)
		if err != nil {
			t.Fatalf("%s: %v", file.Name(), err)
		}
		if !bytes.Equal(formatted, out) {
			t.Errorf("%s: suggested fix is not gofmt-clean", file.Name())
		}
	}
}
