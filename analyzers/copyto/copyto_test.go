package copyto

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

func init() { linelen.Config.Len = 79 }

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestIndentLineDirective(t *testing.T) {
	const src = "package p\n\n//line other.go:999999\nfunc f() {\n\tfor {\n\t}\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{
		Fset:     fset,
		ReadFile: func(string) ([]byte, error) { return []byte(src), nil },
	}
	var loop *ast.ForStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if found, ok := n.(*ast.ForStmt); ok {
			loop = found
		}
		return true
	})
	if loop == nil {
		t.Fatal("no loop")
	}
	lead, ok := indent(pass, loop.Pos())
	if !ok || lead != "\t" {
		t.Fatalf("indent = %q, %v", lead, ok)
	}
}
