package deleteguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

const (
	guard = "\tif _, found := rows[name]; found {\n" + "\t\tdelete(rows, name)\n" +
		"\t}"
	chain = "\tif first {\n" + "\t\tuse()\n" + "\t} else if ok {\n" +
		"\t\tuse()\n" +
		"\t}"
)

// TestFits checks when a fix may write the delete on one line.
func TestFits(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		src     string
		text    string
		chained bool
		err     error
		want    bool
	}{{
		name:  "within the width",
		width: 79,
		src:   program(guard),
		text:  "delete(rows, name)",
		want:  true,
	}, {
		name:  "the rewrite crosses the width",
		width: 40,
		src:   program(guard),
		text:  "delete(veryLongMapName, veryLongKeyName)",
		want:  false,
	}, {
		name:  "the line is already past the width",
		width: 10,
		src:   program(guard),
		text:  "delete(rows, name)",
		want:  true,
	}, {
		name:  "no width set",
		width: 0,
		src:   program(guard),
		text:  "delete(veryLongMapName, veryLongKeyName)",
		want:  true,
	}, {
		name:  "the file cannot be read",
		width: 79,
		src:   program(guard),
		text:  "delete(rows, name)",
		err:   fmt.Errorf("unreadable"),
		want:  false,
	}, {
		name:    "a chained rewrite crosses the width",
		width:   40,
		src:     program(chain),
		text:    "delete(veryLongMapName, veryLongKeyName)",
		chained: true,
		want:    false,
	}, {
		name:  "a last line without a newline",
		width: 100,
		src: "package a\n\nfunc f() { if _, found := rows[name]; found " +
			"{ delete(rows, name) } }",
		text: "delete(rows, name)",
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := linelen.Config.Len
			linelen.Config.Len = tt.width
			t.Cleanup(func() { linelen.Config.Len = previous })
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "a.go", tt.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			pass := &analysis.Pass{
				Fset: fset,
				ReadFile: func(string) ([]byte, error) {
					return []byte(tt.src), tt.err
				},
			}
			ifStmt := lastIf(file)
			if got := fits(pass, ifStmt, tt.text, tt.chained); got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

func lastIf(file *ast.File) *ast.IfStmt {
	var last *ast.IfStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			last = ifStmt
		}
		return true
	})
	return last
}

func program(statement string) string {
	return "package a\n\nfunc f() {\n" + statement + "\n}\n"
}
