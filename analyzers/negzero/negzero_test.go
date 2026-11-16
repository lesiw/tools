package negzero

import (
	"embed"
	"fmt"
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
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestEditWithoutSource(t *testing.T) {
	const src = "package p\n\nvar _ = -0.0\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var unary *ast.UnaryExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok {
			unary = u
			return false
		}
		return true
	})
	if unary == nil {
		t.Fatal("no unary expression in source")
	}
	check := func(name string, read func(string) ([]byte, error)) {
		t.Run(name, func(t *testing.T) {
			var (
				pass = &analysis.Pass{Fset: fset, ReadFile: read}
				got  = edit(pass, unary)
			)
			if want := unary.OpPos; got.Pos != want {
				t.Errorf("Pos = %v, want %v", got.Pos, want)
			}
			if want := unary.OpPos + 1; got.End != want {
				t.Errorf("End = %v, want %v", got.End, want)
			}
			if got, want := string(got.NewText), " "; got != want {
				t.Errorf("NewText = %q, want %q", got, want)
			}
		})
	}
	check("read error", func(string) ([]byte, error) {
		return nil, fmt.Errorf("no source")
	})
	check("short source", func(string) ([]byte, error) { return nil, nil })
}
