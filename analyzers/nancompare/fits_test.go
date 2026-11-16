package nancompare

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

const program = "package a\n\nvar _ = x == y\n"

// TestFits checks when a rewrite may replace a comparison. The fixture pins
// one width, and its // want comment is most of the line, so the widths live
// here.
func TestFits(t *testing.T) {
	tests := []struct {
		name  string
		width int
		text  string
		want  bool
	}{{
		name:  "within the width",
		width: 79,
		text:  "math.IsNaN(x)",
		want:  true,
	}, {
		name:  "the rewrite crosses the width",
		width: 19,
		text:  "math.IsNaN(x)",
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		text:  "math.IsNaN(x)",
		want:  true,
	}, {
		name:  "no width",
		width: 0,
		text:  "math.IsNaN(x)",
		want:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := linelen.Config.Len
			linelen.Config.Len = tt.width
			t.Cleanup(func() { linelen.Config.Len = prev })
			src := []byte(program)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "a.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			decl := f.Decls[0].(*ast.GenDecl)
			spec := decl.Specs[0].(*ast.ValueSpec)
			bin := spec.Values[0].(*ast.BinaryExpr)
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return src, nil
				},
			}
			got := fits(pass, bin.Pos(), bin.End(), tt.text)
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}
