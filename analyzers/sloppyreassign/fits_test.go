package sloppyreassign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

const program = "package a\n\nfunc f() {\n\tif x = g(); x > 0 {\n\t}\n}\n"

// TestFits checks when the header may grow by the byte the fix adds. The
// fixture pins one width, and its // want comment is most of the line, so the
// widths live here.
func TestFits(t *testing.T) {
	tests := []struct {
		name  string
		width int
		src   string
		want  bool
	}{{
		name:  "within the width",
		width: 79,
		src:   program,
		want:  true,
	}, {
		name:  "the fix crosses the width",
		width: 23,
		src:   program,
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		src:   program,
		want:  true,
	}, {
		name:  "no width",
		width: 0,
		src:   program,
		want:  true,
	}, {
		name:  "line directive",
		width: 23,
		src:   "//line a.go:9000\n" + program,
		want:  false,
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
			stmt := decl.Body.List[0].(*ast.IfStmt)
			assign := stmt.Init.(*ast.AssignStmt)
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return []byte(tt.src), nil
				},
			}
			got := fits(pass, &assignment{stmt: assign})
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}
