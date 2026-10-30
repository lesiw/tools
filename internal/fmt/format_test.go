package fmt

import (
	"bytes"
	stdfmt "fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// corpus returns the Go sources the properties are checked against: this
// module's own tree, and the fixtures the goldens are built from.
func corpus(t *testing.T) (paths []string) {
	t.Helper()
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, inputs...)
	walk := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != "../.." {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			paths = append(paths, p)
		}
		return nil
	}
	if err := filepath.WalkDir("../..", walk); err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("empty corpus")
	}
	return paths
}

// read returns the file at name.
func read(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// TestFormatProperties holds the formatter to the claims its callers rest on.
// One pass reaches a fixed point. The output is what gofmt leaves. No comment
// is lost.
func TestFormatProperties(t *testing.T) {
	for _, path := range corpus(t) {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			src := read(t, path)
			out, err := FormatFile(path, src)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			if clean, err := format.Source(out); err != nil {
				t.Fatalf("output does not parse: %v", err)
			} else if !bytes.Equal(clean, out) {
				t.Error("output is not gofmt-clean")
			}
			again, err := Format(out)
			if err != nil {
				t.Fatalf("Format of its own output: %v", err)
			}
			if !bytes.Equal(again, out) {
				t.Error("a second pass changed the output")
			}
			if got, want := comments(out), comments(src); got != want {
				t.Errorf("comments changed:\ngot  %q\nwant %q", got, want)
			}
		})
	}
}

// TestNoAddedLongLine holds the formatter to the rule the width check enforces
// downstream: it may leave a long line it was handed, and it may not make one.
func TestNoAddedLongLine(t *testing.T) {
	for _, path := range corpus(t) {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			src := read(t, path)
			out, err := FormatFile(path, src)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			if got, want := longLines(t, out), longLines(t, src); got > want {
				t.Errorf("long lines %d -> %d", want, got)
			}
		})
	}
}

// longLines returns how many lines the width rules reject.
func longLines(t *testing.T, src []byte) int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return len(cfg.CheckFile(fset, file, src))
}

// comments returns each comment's text with its whitespace collapsed.
func comments(src []byte) string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		return ""
	}
	var out []string
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			out = append(out, strings.Join(strings.Fields(c.Text), " "))
		}
	}
	return strings.Join(out, " ")
}

// TestASTKept holds the formatter to the claim that makes it safe to gate CI
// on. A layout rule never changes what the program says. The dump skips
// positions and comment groups. A legal layout change then compares equal, and
// a structural change shows up.
func TestASTKept(t *testing.T) {
	for _, path := range corpus(t) {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			src := read(t, path)
			out, err := FormatFile(path, src)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			want, err := dumpAST(src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, err := dumpAST(out)
			if err != nil {
				t.Fatalf("parse of the output: %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("the output is a different program:\n%s", firstDifference(want, got))
			}
		})
	}
}

// dumpAST returns a position-free, comment-free dump of src, one entry per
// node and field.
func dumpAST(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []string
	dumpValue(&out, "", reflect.ValueOf(file))
	return out, nil
}

var (
	posType = reflect.TypeFor[token.Pos]()
	cgType  = reflect.TypeFor[*ast.CommentGroup]()
)

func dumpValue(out *[]string, path string, v reflect.Value) {
	if !v.IsValid() || v.Type() == posType || v.Type() == cgType {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			*out = append(*out, path+"=nil")
			return
		}
		dumpValue(out, path, v.Elem())
	case reflect.Struct:
		switch v.Type().Name() {
		case "Object", "Scope":
			// Resolved identifiers point back into the tree.
			return
		}
		*out = append(*out, path+"::"+v.Type().String())
		for i := range v.NumField() {
			f := v.Type().Field(i)
			switch f.Name {
			case "Comments", "Doc", "Comment":
				continue
			}
			if f.PkgPath != "" {
				continue
			}
			dumpValue(out, path+"."+f.Name, v.Field(i))
		}
	case reflect.Slice:
		for i := range v.Len() {
			dumpValue(out, stdfmt.Sprintf("%s[%d]", path, i), v.Index(i))
		}
	default:
		*out = append(*out, stdfmt.Sprintf("%s=%v", path, v.Interface()))
	}
}

// firstDifference renders the first entry at which two dumps disagree.
func firstDifference(want, got []string) string {
	for i := range min(len(want), len(got)) {
		if want[i] != got[i] {
			return stdfmt.Sprintf("  got  %s\n  want %s", got[i], want[i])
		}
	}
	if len(got) > len(want) {
		return "  extra node: " + got[len(want)]
	}
	return "  missing node: " + want[len(got)]
}
