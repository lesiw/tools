package errorsastypeshadow

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

// Checks the exemption for the packages whose own tests write incorrect
// programs.
func TestExempt(t *testing.T) {
	cases := []struct{ name, path, src string }{{
		name: "errors",
		path: "errors",
		src: `package errors

type boxError struct{ error }

func AsType[E error](err error) (E, bool) {
	var zero E
	return zero, false
}

func f(err error) {
	if err, ok := AsType[*boxError](err); ok {
		_ = err
	} else if err, ok := AsType[*boxError](err); ok {
		_ = err
	}
}
`,
	}, {
		name: "errors_test",
		path: "errors_test",
		src: `package errors_test

import "errors"

type boxError struct{ error }

func f(err error) {
	if err, ok := errors.AsType[*boxError](err); ok {
		_ = err
	} else if err, ok := errors.AsType[*boxError](err); ok {
		_ = err
	}
}
`,
	}}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var (
				fset = token.NewFileSet()
				info = &types.Info{
					Types: make(map[ast.Expr]types.TypeAndValue),
					Defs:  make(map[*ast.Ident]types.Object),
					Uses:  make(map[*ast.Ident]types.Object),
				}
			)
			file, err := parser.ParseFile(fset, tt.name+".go", tt.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			conf := types.Config{Importer: importer.Default()}
			pkg, err := conf.Check(tt.path, fset, []*ast.File{file}, info)
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
				t.Errorf(
					"run reported %d diagnostics in package %s, want 0",
					found, tt.path,
				)
			}
		})
	}
}
