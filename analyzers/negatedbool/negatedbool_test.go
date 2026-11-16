package negatedbool

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

func TestAnalysisTestDot(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "dot")
}

func TestAnalysisTestWidth(t *testing.T) {
	var (
		dir   = fixtures.Dir(t, testdata)
		limit = linelen.Config.Len
	)
	linelen.Config.Len = 79
	defer func() { linelen.Config.Len = limit }()
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "width")
}

func TestFits(t *testing.T) {
	const src = "package p\n\nfunc f() bool {\n\treturn !(!(\n\t\tfalse))\n}\n"
	name := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(name, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var outer *ast.UnaryExpr
	ast.Inspect(file, func(n ast.Node) bool {
		u, ok := n.(*ast.UnaryExpr)
		if !ok || u.Op != token.NOT {
			return true
		}
		inner, ok := ast.Unparen(u.X).(*ast.UnaryExpr)
		if ok && inner.Op == token.NOT {
			outer = u
			return false
		}
		return true
	})
	if outer == nil {
		t.Fatal("no double negation in source")
	}
	pass := &analysis.Pass{Fset: fset, ReadFile: os.ReadFile}
	limit := linelen.Config.Len
	linelen.Config.Len = 79
	defer func() { linelen.Config.Len = limit }()
	if !fits(pass, outer, "false") {
		t.Error("short replacement does not fit")
	}
	if fits(pass, outer, strings.Repeat("x", 200)) {
		t.Error("long replacement fits")
	}
}
