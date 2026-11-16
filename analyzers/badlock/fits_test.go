package badlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

// TestFits checks when a fix may rewrite the deferred method. The fixture runs
// at one width, and a // want on the deferred call would widen the line the
// fix is measured against, so the width cases live here.
func TestFits(t *testing.T) {
	tests := []struct {
		name  string
		width int
		src   string
		err   error
		want  bool
	}{{
		name:  "within the width",
		width: 79,
		src:   program("\tmu.Lock()"),
		want:  true,
	}, {
		name:  "the rewrite crosses the width",
		width: 14,
		src:   program("\tmu.Lock()"),
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		src:   program("\tmu.Lock()"),
		want:  true,
	}, {
		name:  "no width set",
		width: 0,
		src:   program("\tmu.Lock()"),
		want:  true,
	}, {
		name:  "the file cannot be read",
		width: 79,
		src:   program("\tmu.Lock()"),
		err:   fmt.Errorf("unreadable"),
		want:  false,
	}, {
		name:  "line directive",
		width: 79,
		src:   "//line :9000:1\n" + program("\tmu.Lock()"),
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
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return []byte(tt.src), tt.err
				},
			}
			name := sel(f)
			got := fits(pass, name.Pos(), name.End(), "Unlock")
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

// sel returns the method name the parsed program's only statement calls.
func sel(f *ast.File) *ast.Ident {
	var (
		block = f.Decls[len(f.Decls)-1].(*ast.FuncDecl).Body
		stmt  = block.List[0].(*ast.ExprStmt)
		call  = stmt.X.(*ast.CallExpr)
	)
	return call.Fun.(*ast.SelectorExpr).Sel
}

func program(statement string) string {
	return "package a\n\ntype M struct{}\n\nfunc f(mu *M) {\n" + statement + "\n}\n"
}
