package sortnosort

import (
	"embed"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"lesiw.io/tools/internal/fixtures"
)

//go:embed testdata
var testdata embed.FS

func TestAnalysisTest(t *testing.T) {
	dir := fixtures.Dir(t, testdata)
	analysistest.RunWithSuggestedFixes(
		t, dir, Analyzer, "a", "aliasdup", "aliasuse", "blank",
		"blankdup", "blankimplicit", "dot", "dotalias",
		"dotaliasdup", "dotaliasshadow", "dotaliasshadowbare",
		"shadowmissing",
		"dotaliasfield", "dotaliasmixed", "dotaliasused",
		"dotdupsort",
	)
}

func sortDependencyPaths(t *testing.T) ([]string, string) {
	t.Helper()
	sortPkg, err := build.Default.Import("sort", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	direct := append([]string(nil), sortPkg.Imports...)
	known := make(map[string]struct{}, len(direct))
	for _, path := range direct {
		known[path] = struct{}{}
	}
	for _, path := range direct {
		pkg, err := build.Default.Import(path, "", 0)
		if err != nil {
			continue
		}
		for _, dep := range pkg.Imports {
			if _, ok := known[dep]; !ok && dep != "sort" {
				return direct, dep
			}
		}
	}
	t.Fatal("no transitive dependency of sort found")
	return nil, ""
}

func TestSortDependencies(t *testing.T) {
	direct, transitive := sortDependencyPaths(t)
	for _, path := range direct {
		if !sortDep(path) {
			t.Errorf("%s: not read as a dependency of sort", path)
		}
	}
	if !sortDep("sort") {
		t.Error("sort: not read as a dependency of sort")
	}
	if !sortDep(transitive) {
		t.Errorf("%s: not read as a transitive dependency of sort", transitive)
	}
	if sortDep("example.com/m") {
		t.Error("example.com/m: read as a dependency of sort")
	}
}

func TestSuggestedFixes(t *testing.T) {
	var (
		dir     = fixtures.Dir(t, testdata)
		results = analysistest.Run(
			t, dir, Analyzer, "a", "aliasdup", "aliasuse",
			"blankdup", "blankimplicit", "dot", "dotaliasmixed",
			"dotaliasshadow", "dotaliasshadowbare",
			"dotaliasused", "dotdupsort",
		)
		counts = make(map[string]int)
	)
	for _, result := range results {
		for _, d := range result.Diagnostics {
			for _, fix := range d.SuggestedFixes {
				pkg := result.Pass.Pkg.Path()
				counts[pkg]++
				if !fixMessage(fix.Message) {
					t.Errorf("fix message is %q", fix.Message)
				}
				if pkg == "aliasuse" && len(fix.TextEdits) != 2 {
					t.Errorf("import-adding fix has %d edits, want 2", len(fix.TextEdits))
				}
			}
		}
	}
	want := map[string]int{
		"aliasdup":           1,
		"blankdup":           1,
		"blankimplicit":      1,
		"dotaliasshadow":     1,
		"dotaliasshadowbare": 1,
		"dotaliasmixed":      1,
		"dotaliasused":       1,
		"dotdupsort":         2,
	}
	packages := []string{
		"a", "aliasdup", "aliasuse", "blankdup",
		"blankimplicit", "dot", "dotaliasmixed",
		"dotaliasshadow", "dotaliasshadowbare",
		"dotaliasused", "dotdupsort",
	}
	for _, pkg := range packages {
		if counts[pkg] == 0 {
			t.Errorf("package %s offered no fix", pkg)
		}
		if n, ok := want[pkg]; ok && counts[pkg] != n {
			t.Errorf(
				"package %s offered %d fixes, want %d",
				pkg, counts[pkg], n,
			)
		}
	}
}

func fixMessage(message string) bool {
	return strings.HasPrefix(message, "Write ") &&
		strings.HasSuffix(message, " (changes behavior)")
}

func TestReplacementSortDependency(t *testing.T) {
	var (
		direct, transitive = sortDependencyPaths(t)
		cases              = []struct {
			path string
			text string
			ok   bool
		}{
			{direct[0], "", false},
			{"sort", "", false},
			{transitive, "", false},
			{"example.com/m", "sort.Strings(names)", true},
		}
	)
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			var (
				target = ast.NewIdent("names")
				assign = &ast.AssignStmt{
					Lhs: []ast.Expr{target},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun:  ast.NewIdent("StringSlice"),
						Args: []ast.Expr{ast.NewIdent("names")},
					}},
				}
				pkg = types.NewPackage(tc.path, "p")
			)
			pkg.Scope().Insert(types.NewVar(
				token.NoPos, pkg, "names",
				types.NewSlice(types.Typ[types.String]),
			))
			pkg.MarkComplete()
			fset := token.NewFileSet()
			tv, err := types.Eval(fset, pkg, token.NoPos, "names")
			if err != nil {
				t.Fatal(err)
			}
			pass := &analysis.Pass{
				Pkg:  pkg,
				Fset: fset,
				TypesInfo: &types.Info{
					Types: map[ast.Expr]types.TypeAndValue{target: tv},
				},
			}
			file := &ast.File{Name: ast.NewIdent("a")}
			call := assign.Rhs[0].(*ast.CallExpr)
			text, ok := replacement(
				pass, file, nil, nil, nil, call, assign,
				target, "Strings",
			)
			if text != tc.text || ok != tc.ok {
				t.Errorf(
					"replacement is %q, %t; want %q, %t",
					text, ok, tc.text, tc.ok,
				)
			}
			if !ok {
				var (
					s = site{
						assign: assign, target: target, call: call,
						sortFunc: "Strings", typeName: "sort.StringSlice",
						text: text, fixable: ok,
					}
					d = diagnose(pass, file, nil, nil, s, []site{s})
				)
				if len(d.SuggestedFixes) > 0 {
					t.Error("diagnose offers a fix")
				}
				if strings.Contains(d.Message, "; write ") {
					t.Errorf("unfixable diagnostic names a rewrite: %q", d.Message)
				}
			}
		})
	}
}
