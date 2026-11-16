package wrapperarg

import (
	"embed"
	"go/ast"
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
	analysistest.RunWithSuggestedFixes(t, dir, Analyzer, "a")
}

// TestOwnDeclaration covers the guard that keeps a wrapper body from being
// rewritten into a recursive call. No fixture can declare the standard
// library's wrapper, so the test swaps in a table entry for a local method.
func TestOwnDeclaration(t *testing.T) {
	const src = `package p

type counter struct{}

func (counter) Add(n int) {}

func (counter) Done() {
	var c counter
	c.Add(-1)
	func() { c.Add(-1) }()
}

func (counter) Other() {
	var c counter
	c.Add(-1)
}
`
	var (
		fset = token.NewFileSet()
		info = &types.Info{
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		}
		reported []analysis.Diagnostic
	)
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := new(types.Config).Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	var calls []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Add" {
			calls = append(calls, call)
		}
		return true
	})
	if len(calls) != 3 {
		t.Fatalf("found %d calls, want 3", len(calls))
	}
	saved := wrappers
	wrappers = []wrapper{{
		call:   "(p.counter).Add",
		want:   minusOne,
		target: "Done",
		drop:   []int{0},
		note:   "a delta of -1",
	}}
	t.Cleanup(func() { wrappers = saved })
	pass := &analysis.Pass{
		Fset:      fset,
		Pkg:       pkg,
		TypesInfo: info,
		Report: func(d analysis.Diagnostic) {
			reported = append(reported, d)
		},
	}
	check(pass, file, calls[0])
	check(pass, file, calls[1])
	if len(reported) != 0 {
		t.Errorf("the target's body was reported: %v", reported)
	}
	check(pass, file, calls[2])
	if len(reported) != 1 {
		t.Fatalf("%d diagnostics, want 1", len(reported))
	}
	want := "counter.Add with a delta of -1 can be replaced by counter.Done"
	if got := reported[0].Message; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if len(reported[0].SuggestedFixes) != 1 {
		t.Fatal("the call outside the target has no fix")
	}
	edits := reported[0].SuggestedFixes[0].TextEdits
	if len(edits) != 2 {
		t.Fatalf("%d edits, want 2", len(edits))
	}
	if got := string(edits[0].NewText); got != "Done" {
		t.Errorf("rename edit = %q, want %q", got, "Done")
	}
	if len(edits[1].NewText) != 0 {
		t.Errorf("delete edit writes %q, want nothing", edits[1].NewText)
	}
}
