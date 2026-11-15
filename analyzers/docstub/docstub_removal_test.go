package docstub

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestRemoval checks when a comment may be deleted whole. The fixture cannot
// reach the case where the text on disk is not the text that was parsed, so it
// lives here.
func TestRemoval(t *testing.T) {
	tests := []struct {
		name  string
		src   string // the text the analyzer would read
		parse string // the text it parsed, when the two differ
		want  bool
	}{{
		name: "own line",
		src:  "package a\n\n// Foo does foo.\nfunc Foo() {}\n",
		want: true,
	}, {
		name:  "shortened since it was parsed",
		src:   "package a\n",
		parse: "package a\n\n// Foo does foo.\nfunc Foo() {}\n",
		want:  false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parse := tt.parse
			if parse == "" {
				parse = tt.src
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(
				fset, "a.go", parse,
				parser.ParseComments,
			)
			if err != nil {
				t.Fatal(err)
			}
			decl := f.Decls[len(f.Decls)-1].(*ast.FuncDecl)
			_, got := removal(fset.File(f.FileStart), []byte(tt.src), decl.Doc)
			if got != tt.want {
				t.Errorf("removal ok = %v, want %v", got, tt.want)
			}
		})
	}
}
