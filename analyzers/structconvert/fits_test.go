package structconvert

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"lesiw.io/tools/analyzers/linelen"
)

const longName = "aDestinationStructTypeNameThatIsLong"

func TestFits(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  bool
	}{
		{"within the width", 100, true},
		{"the rewrite crosses the width", 79, false},
		{"the line is already past the width", 10, true},
		{"no width set", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := linelen.Config.Len
			linelen.Config.Len = tt.width
			t.Cleanup(func() { linelen.Config.Len = prev })
			src := program()
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "a.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			lit := elided(f)
			if lit == nil {
				t.Fatal("no elided literal")
			}
			got := fits([]byte(src), fset.File(f.Pos()), lit, longName+"(v)")
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

func program() string {
	lines := []string{
		"package p",
		"",
		"type " + longName + " struct {",
		"\tKind, Note string",
		"}",
		"",
		"func f(v " + longName + ") []" + longName + " {",
		"\treturn []" + longName + "{{Kind: v.Kind, Note: v.Note}}",
		"}",
	}
	return strings.Join(lines, "\n") + "\n"
}

func elided(f *ast.File) *ast.CompositeLit {
	var found *ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && lit.Type == nil {
			found = lit
		}
		return true
	})
	return found
}

func TestFitsAcrossLines(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  bool
	}{
		{"the rewrite fits", 30, true},
		{"the rewrite crosses the width", 24, false},
		{"a line in the span is already past the width", 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := linelen.Config.Len
			linelen.Config.Len = tt.width
			t.Cleanup(func() { linelen.Config.Len = prev })
			src := across()
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "a.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			lit := explicit(f)
			if lit == nil {
				t.Fatal("no literal")
			}
			got := fits([]byte(src), fset.File(f.Pos()), lit, "destination(v)")
			if got != tt.want {
				t.Errorf("fits = %v, want %v", got, tt.want)
			}
		})
	}
}

func across() string {
	return strings.Join([]string{
		"package p",
		"",
		"type first struct {",
		"\tKind string",
		"}",
		"",
		"type destination struct {",
		"\tKind string",
		"}",
		"",
		"func f(v first) destination {",
		"\treturn destination{",
		"\t\tKind: v.Kind,",
		"\t}",
		"}",
	}, "\n") + "\n"
}

func explicit(f *ast.File) *ast.CompositeLit {
	var found *ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && lit.Type != nil {
			found = lit
		}
		return true
	})
	return found
}

func TestFitsMultilineReplacement(t *testing.T) {
	prev := linelen.Config.Len
	linelen.Config.Len = 24
	t.Cleanup(func() { linelen.Config.Len = prev })
	src := across()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	lit := explicit(f)
	if lit == nil {
		t.Fatal("no literal")
	}
	got := fits(
		[]byte(src), fset.File(f.Pos()),
		lit, "struct {\n\tKind string\n}(v)",
	)
	if !got {
		t.Error("fits = false, want true")
	}
}
