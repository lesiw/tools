package filepathjoin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

// TestFits checks when a rewrite may replace a path. The fixture runs at one
// width, so the width cases live here.
func TestFits(t *testing.T) {
	tests := []struct {
		name  string
		width int
		src   string
		text  string
		want  bool
	}{{
		name:  "within the width",
		width: 79,
		src:   program("\treturn dir + \"/\" + name"),
		text:  "filepath.Join(dir, name)",
		want:  true,
	}, {
		name:  "the rewrite crosses the width",
		width: 30,
		src:   program("\treturn dir + \"/\" + name"),
		text:  "filepath.Join(dir, name)",
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		src:   program("\treturn dir + \"/\" + name"),
		text:  "filepath.Join(dir, name)",
		want:  true,
	}, {
		name:  "across lines",
		width: 79,
		src:   program("\treturn dir +\n\t\t\"/\" + name"),
		text:  "filepath.Join(dir, name)",
		want:  false,
	}, {
		name:  "line directive",
		width: 79,
		src:   "//line :9000:1\n" + program("\treturn dir + \"/\" + name"),
		text:  "filepath.Join(dir, name)",
		want:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := linelen.Config.Len
			linelen.Config.Len = tt.width
			t.Cleanup(func() { linelen.Config.Len = prev })
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "a.go", tt.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			decl := f.Decls[0].(*ast.FuncDecl)
			ret := decl.Body.List[0].(*ast.ReturnStmt)
			bin := ret.Results[0].(*ast.BinaryExpr)
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return []byte(tt.src), nil
				},
			}
			got := fits(pass, bin.Pos(), bin.End(), tt.text)
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

func program(s string) string {
	return "package a\n\nfunc f(dir, name string) string {\n" + s + "\n}\n"
}
