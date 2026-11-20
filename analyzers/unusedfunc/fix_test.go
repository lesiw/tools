package unusedfunc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// TestDeleteDeclText holds each edit to the text it removes. analysistest
// formats the fixed file before comparing it with the golden, so a separator
// left behind on a shared line is invisible there.
func TestDeleteDeclText(t *testing.T) {
	tests := []struct {
		name, src, want string
		at              int // which declaration the fix deletes
	}{{
		name: "alone",
		src:  "package p\n\nvar dead = 1\n",
		want: "var dead = 1",
	}, {
		name: "with a doc comment",
		src:  "package p\n\n// dead is a leftover.\nvar dead = 1\n",
		want: "// dead is a leftover.\nvar dead = 1",
	}, {
		name: "followed on its line",
		src:  "package p\n\nvar dead = 1; var live = 2\n",
		want: "var dead = 1; ",
	}, {
		name: "preceded on its line",
		src:  "package p\n\nvar live = 1; var dead = 2\n",
		want: "var dead = 2",
		at:   1,
	}, {
		name: "last of three on its line",
		src:  "package p\n\nvar one = 1; var two = 2; var dead = 3\n",
		want: "var dead = 3",
		at:   2,
	}, {
		name: "between two on its line",
		src:  "package p\n\nvar one = 1; var dead = 2; var two = 3\n",
		want: "var dead = 2; ",
		at:   1,
	}, {
		name: "with a comment after it",
		src:  "package p\n\nvar dead = 1 // gone\n",
		want: "var dead = 1 // gone",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				fset, file = parse(t, tt.src)
				decl       = file.Decls[tt.at]
				got        = removed(
					t, fset, tt.src,
					deleteDecl(fset.File(file.Pos()), file, decl),
				)
			)
			if got != tt.want {
				t.Errorf("removed %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDeleteSpecText holds the edits that remove one specification of a block,
// which a golden cannot show either: the formatter reflows the block before it
// is compared.
func TestDeleteSpecText(t *testing.T) {
	tests := []struct {
		name, src, want string
		at              int // which specification the fix deletes
	}{{
		name: "first",
		src:  "package p\n\nvar (dead = 1; b = 2; c = 3)\n",
		want: "dead = 1; ",
	}, {
		name: "middle",
		src:  "package p\n\nvar (a = 1; dead = 2; c = 3)\n",
		want: "dead = 2; ",
		at:   1,
	}, {
		name: "last",
		src:  "package p\n\nvar (a = 1; b = 2; dead = 3)\n",
		want: "dead = 3",
		at:   2,
	}, {
		name: "last with a doc comment",
		src:  "package p\n\nvar (\n\ta = 1\n\t// deadDoc.\n\tdead = 2\n)\n",
		want: "// deadDoc.\n\tdead = 2",
		at:   1,
	}, {
		name: "last below a comment",
		src:  "package p\n\nvar (\n\ta = 1 // keep\n\tdead = 2\n)\n",
		want: "dead = 2",
		at:   1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				fset, file = parse(t, tt.src)
				decl       = file.Decls[0].(*ast.GenDecl)
				got        = removed(
					t, fset, tt.src,
					deleteSpec(fset.File(file.Pos()), file, decl, tt.at),
				)
			)
			if got != tt.want {
				t.Errorf("removed %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDropText holds the edits that remove one name from a specification,
// where the comma is the residue a formatter would hide.
func TestDropText(t *testing.T) {
	tests := []struct {
		name, src, want string
		at              int // which name the fix removes
		paired          bool
	}{
		{"name first", "package p\n\nvar (a, b, c int)\n", "a, ", 0, false},
		{"name last", "package p\n\nvar (a, b, c int)\n", ", c", 2, false},
		{"pair first", "package p\n\nvar (a, b = 1, 2)\n", "a, 1, ", 0, true},
		{"pair last", "package p\n\nvar (a, b = 1, 2)\n", ", b, 2", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				fset, file = parse(t, tt.src)
				spec       = valueSpec(t, file)
				edits      = dropName(spec, tt.at)
			)
			if tt.paired {
				edits = dropPair(spec, tt.at)
			}
			got := removed(t, fset, tt.src, edits)
			if got != tt.want {
				t.Errorf("removed %q, want %q", got, tt.want)
			}
		})
	}
}

// valueSpec returns the only specification of the first declaration in file.
func valueSpec(t *testing.T, file *ast.File) *ast.ValueSpec {
	t.Helper()
	decl, ok := file.Decls[0].(*ast.GenDecl)
	if !ok || len(decl.Specs) != 1 {
		t.Fatal("the source does not hold one declaration of one spec")
	}
	return decl.Specs[0].(*ast.ValueSpec)
}

// parse returns the file set and file for src, with its comments.
func parse(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p/p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return fset, file
}

// removed returns the text the edits delete from src.
func removed(t *testing.T, fset *token.FileSet, src string, edits []analysis.TextEdit) (out string) {
	t.Helper()
	if len(edits) == 0 {
		t.Fatal("no edits")
	}
	bounds := fset.File(edits[0].Pos)
	for _, edit := range edits {
		out += src[bounds.Offset(edit.Pos):bounds.Offset(edit.End)]
	}
	return out
}
