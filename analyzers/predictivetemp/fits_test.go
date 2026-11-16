package predictivetemp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

// line is the body shared by the cases that differ only in width.
const line = "\treturn os.Create(\"/tmp/lock\")"

// TestFits checks when a rewrite may replace a call. The fixture runs at one
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
		src:   program(line),
		text:  `os.CreateTemp("/tmp", "lock")`,
		want:  true,
	}, {
		name:  "the rewrite crosses the width",
		width: 36,
		src:   program(line),
		text:  `os.CreateTemp("/tmp", "lock")`,
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		src:   program(line),
		text:  `os.CreateTemp("/tmp", "lock")`,
		want:  true,
	}, {
		name:  "across lines",
		width: 79,
		src:   program("\treturn os.Create(\n\t\t\"/tmp/lock\")"),
		text:  `os.CreateTemp("/tmp", "lock")`,
		want:  false,
	}, {
		name:  "line directive",
		width: 79,
		src:   "//line :9000:1\n" + program(line),
		text:  `os.CreateTemp("/tmp", "lock")`,
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
			// The case's function is the last declaration; an import
			// comes first.
			decl := f.Decls[len(f.Decls)-1].(*ast.FuncDecl)
			ret := decl.Body.List[0].(*ast.ReturnStmt)
			call := ret.Results[0].(*ast.CallExpr)
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return []byte(tt.src), nil
				},
			}
			got := fits(pass, call.Pos(), call.End(), tt.text)
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

// program builds a file whose last declaration is the function f.
func program(s string) string {
	return "package a\n\nimport \"os\"\n\nfunc f() (*os.File, error) {\n" + s + "\n}\n"
}
