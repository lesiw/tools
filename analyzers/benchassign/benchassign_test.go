package benchassign

import (
	"embed"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.Run(t, dir, Analyzer, "a")
}

// TestTestingPackage checks the exemption for the package that sets N.
func TestTestingPackage(t *testing.T) {
	const src = `package testing

type B struct{ N int }

func f(b *B) {
	b.N = 1
}
`
	var (
		fset = token.NewFileSet()
		info = &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		}
	)
	file, err := parser.ParseFile(fset, "testing.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("testing", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	var found int
	pass := &analysis.Pass{
		Analyzer:  Analyzer,
		Fset:      fset,
		Files:     []*ast.File{file},
		Pkg:       pkg,
		TypesInfo: info,
		Report:    func(analysis.Diagnostic) { found++ },
	}
	if _, err := run(pass); err != nil {
		t.Fatal(err)
	}
	if found != 0 {
		t.Errorf("run reported %d diagnostics in package testing, want 0", found)
	}
}
