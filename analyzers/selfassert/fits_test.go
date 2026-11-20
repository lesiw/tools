package selfassert

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

// TestFits checks when a rewrite may replace an assertion. The fixture runs at
// one width, and the width and line cases live here.
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
		src:   program("\tcurrent.(filter)"),
		text:  "current, current != nil",
		want:  true,
	}, {
		name:  "the rewrite crosses the width",
		width: 24,
		src:   program("\tcurrent.(filter)"),
		text:  "current, current != nil",
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		src:   program("\tcurrent.(filter)"),
		text:  "current, current != nil",
		want:  true,
	}, {
		name:  "across lines",
		width: 79,
		src:   program("\tcurrent.\n\t\t(filter)"),
		text:  "current, current != nil",
		want:  false,
	}, {
		name:  "no width",
		width: 0,
		src:   program("\tcurrent.\n\t\t(filter)"),
		text:  "current, current != nil",
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
			var expr ast.Expr
			ast.Inspect(f, func(n ast.Node) bool {
				if assert, ok := n.(*ast.TypeAssertExpr); ok {
					expr = assert
				}
				return expr == nil
			})
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return []byte(tt.src), nil
				},
			}
			got := fits(pass, expr.Pos(), expr.End(), tt.text)
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

func program(s string) string {
	return "package a\n\ntype filter interface{ allow() bool }\n\n" +
		"var current filter\n\nfunc f() {\n" + s + "\n}\n"
}
