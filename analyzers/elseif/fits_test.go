package elseif

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"lesiw.io/tools/analyzers/linelen"
)

// The fixtures run at the one width the package sets, so the boundary and the
// no-width case are measured here.

const (
	one  = "\t\tif b {\n\t\t\tc()\n\t\t}"
	two  = "\t\tif b ||\n\t\t\tc() {\n\t\t\td()\n\t\t}"
	note = " // note"
)

func TestFitsLine(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		nested string
		want   bool
	}{
		{"at the width", 25, one, true},
		{"past the width", 24, one, false},
		{"no width set", 0, one, true},
		{"header across lines", 79, two, false},
		{"header across lines, no width set", 0, two, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			prev := linelen.Config.Len
			linelen.Config.Len = test.width
			t.Cleanup(func() { linelen.Config.Len = prev })
			got := fitsLineAt(t, test.nested)
			if got != test.want {
				t.Errorf("fitsLine = %v, want %v", got, test.want)
			}
		})
	}
}

// wrapper holds the nested if in the else block of a function.
const wrapper = "package a\n\nfunc f() {\n\tif a {\n\t} else {\n%s\n\t}\n}\n"

// fitsLineAt reports whether the comment fits beside the nested if of nested,
// which is wrapped in the else block of a function.
func fitsLineAt(t *testing.T, nested string) bool {
	t.Helper()
	var (
		src       = fmt.Sprintf(wrapper, nested)
		fset      = token.NewFileSet()
		file, err = parser.ParseFile(fset, "a.go", src, 0)
	)
	if err != nil {
		t.Fatal(err)
	}
	stmt, inner := nestedIf(t, file)
	return fitsLine([]byte(src), fset.File(inner.Pos()), stmt, inner, note)
}

// nestedIf returns the if of file that may be written as else if, and the if
// nested in its else block.
func nestedIf(t *testing.T, file *ast.File) (stmt, inner *ast.IfStmt) {
	t.Helper()
	ast.Inspect(file, func(n ast.Node) bool {
		if stmt != nil {
			return false
		}
		if _, ok := n.(*ast.IfStmt); !ok {
			return true
		}
		stmt = n.(*ast.IfStmt)
		block, ok := stmt.Else.(*ast.BlockStmt)
		if !ok || len(block.List) != 1 {
			stmt = nil
			return true
		}
		inner, ok = block.List[0].(*ast.IfStmt)
		if !ok {
			stmt = nil
			return true
		}
		return false
	})
	if stmt == nil || inner == nil {
		t.Fatal("no nested if")
	}
	return stmt, inner
}
