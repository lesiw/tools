package sliceshift

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

// The commands set the width they run with; the tests run what the commands
// run.
func init() { linelen.Config.Len = 79 }

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestIntLimit checks the bound against the target's int, with 32-bit sizes so
// a subtraction chain can pass the individual literals and still overflow.
func TestIntLimit(t *testing.T) {
	sizes := types.SizesFor("gc", "386")
	if sizes.Sizeof(types.Typ[types.Int]) != 4 {
		t.Fatal("386 sizes do not give a 32-bit int")
	}
	forward := func(start, stop string) bool {
		src := "package a\n\nfunc f(xs []int) {\n" +
			"\tfor i := " + start + "; i < " + stop + "; i++ {\n" +
			"\t\txs[i] = xs[i+1]\n" +
			"\t}\n}\n"
		return boundedSource(t, src, sizes)
	}
	reverse := func(start, stop string) bool {
		src := "package a\n\nfunc f(xs []int) {\n" +
			"\tfor i := " + start + "; i > " + stop + "; i-- {\n" +
			"\t\txs[i] = xs[i-1]\n" +
			"\t}\n}\n"
		return boundedSource(t, src, sizes)
	}
	if !forward("0", "len(xs)-2147483646") {
		t.Error("a bound within the int limit was rejected")
	}
	if forward("0", "len(xs)-2147483647-100") {
		t.Error("a bound beyond the int limit was accepted")
	}
	if forward("2147483647", "len(xs)-1") {
		t.Error("a start whose sum overflows was accepted")
	}
	if !reverse("len(xs)-1-1", "0") {
		t.Error("a reverse chain within the int limit was rejected")
	}
	if reverse("len(xs)-2147483647-100", "0") {
		t.Error("a reverse chain beyond the int limit was accepted")
	}
	if reverse("len(xs)-1", "2147483647") {
		t.Error("a reverse stop whose successor overflows was accepted")
	}
}

// boundedSource reports whether the bound check accepts the loop in src.
func boundedSource(t *testing.T, src string, sizes types.Sizes) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "a.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:  make(map[ast.Expr]types.TypeAndValue),
		Defs:   make(map[*ast.Ident]types.Object),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}
	pkg, err := (&types.Config{Sizes: sizes}).Check(
		"a", fset,
		[]*ast.File{file}, info,
	)
	if err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{
		Fset:       fset,
		Files:      []*ast.File{file},
		Pkg:        pkg,
		TypesInfo:  info,
		TypesSizes: sizes,
	}
	var loop *ast.ForStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if found, ok := n.(*ast.ForStmt); ok {
			loop = found
			return false
		}
		return true
	})
	if loop == nil {
		t.Fatal("no loop")
	}
	s, ok := shiftOf(pass, loop)
	if !ok {
		t.Fatal("the loop was not recognized as a shift")
	}
	return bounded(pass, s)
}
