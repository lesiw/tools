package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// TestMain registers the shared flags the way main does, so cfg
// carries concrete values and no test path resolves defaults.
func TestMain(m *testing.M) {
	cfg.Flags(flag.NewFlagSet("fmt", flag.ContinueOnError))
	os.Exit(m.Run())
}

// TestGolden formats each testdata/*.input file and compares the
// result to its .golden sibling. A name ending in ".reflow" is
// formatted with doc comment reflowing enabled.
func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no testdata inputs")
	}
	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input")
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(
				strings.TrimSuffix(input, ".input") + ".golden",
			)
			if err != nil {
				t.Fatal(err)
			}
			reflow := strings.HasSuffix(name, ".reflow")
			got := formatted(t, input, reflow, false)
			if string(got) != string(want) {
				t.Errorf("formatSrc mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestGoldenWidth holds every golden to its input's overlong count
// through linelen itself, so the carve-out for declaration signatures
// applies the way the analyzer applies it. A golden may inherit an
// overlong line, such as a trailing comment the formatter leaves for
// linelen to report, but formatting may never add one. A line of tabs
// is short in bytes and long on screen, and every packing decision is
// made in columns; this catches any that slips back into counting
// bytes.
func TestGoldenWidth(t *testing.T) {
	goldens, err := filepath.Glob("testdata/*.golden")
	if err != nil {
		t.Fatal(err)
	}
	if len(goldens) == 0 {
		t.Fatal("no goldens")
	}
	for _, path := range goldens {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		input := strings.TrimSuffix(path, ".golden") + ".input"
		in, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		before, err := overlong(input, in)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		after, err := overlong(path, src)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if after > before {
			t.Errorf("%s: overlong lines %d -> %d", path, before, after)
		}
	}
}

// TestNoAddedOverlong holds the formatter to the rule the linelen
// analyzer enforces downstream: it may leave an overlong line it was
// handed, but it may never make one. Widths come from linelen itself,
// so the carve-out for declaration signatures applies the way the
// analyzer applies it — a line of tabs is short in bytes and long on
// screen, and every packing decision is made in columns.
func TestNoAddedOverlong(t *testing.T) {
	for _, path := range corpus(t) {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			out := formatted(t, path, false, false)
			before, err := overlong(path, src)
			if err != nil {
				t.Fatal(err)
			}
			after, err := overlong(path, out)
			if err != nil {
				t.Fatal(err)
			}
			if after > before {
				t.Errorf("overlong lines %d -> %d", before, after)
			}
		})
	}
}

// overlong counts the lines linelen reports as over the limit.
func overlong(path string, src []byte) (int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	return len(cfg.Check(fset, file, src)), nil
}

// TestProperties holds every corpus file to the invariants that no
// transformation may break: the output is a fixed point, it is
// gofmt-clean, it keeps every comment, and it parses to the same
// position-free AST as the input.
func TestProperties(t *testing.T) {
	files := corpus(t)
	for _, reflow := range []bool{false, true} {
		for _, path := range files {
			name := fmt.Sprintf("%s/reflow=%v", path, reflow)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				checkProperties(t, path, reflow)
			})
		}
	}
}

// formatted returns formatSrc over path's contents, or over that
// result when again is set, cached so the suite's tests share one
// render per corpus file instead of repeating the heaviest work.
func formatted(t *testing.T, path string, reflow, again bool) []byte {
	t.Helper()
	type key struct {
		path          string
		reflow, again bool
	}
	k := key{path, reflow, again}
	if v, ok := formatCache.Load(k); ok {
		return v.([]byte)
	}
	var src []byte
	if again {
		src = formatted(t, path, reflow, false)
	} else {
		var err error
		src, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	out, err := formatSrc(src, reflow)
	if err != nil {
		t.Fatalf("formatSrc %s: %v", path, err)
	}
	formatCache.Store(k, out)
	return out
}

var formatCache sync.Map

func checkProperties(t *testing.T, path string, reflow bool) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := formatted(t, path, reflow, false)
	if clean, err := format.Source(out); err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, out)
	} else if !bytes.Equal(clean, out) {
		t.Errorf("output is not gofmt-clean:\n%s", firstDiff(out, clean))
	}
	again := formatted(t, path, reflow, true)
	if !bytes.Equal(again, out) {
		t.Errorf("not a fixed point:\n%s", firstDiff(out, again))
	}
	in, err := comments(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := comments(out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflow && len(in) != len(got) {
		t.Errorf("comment count: got %d, want %d", len(got), len(in))
	}
	if strings.Join(in, " ") != strings.Join(got, " ") {
		t.Errorf("comment text changed:\nin:  %s\nout: %s",
			strings.Join(in, " "), strings.Join(got, " "),
		)
	}
	wantAST, err := astDump(src)
	if err != nil {
		t.Fatal(err)
	}
	gotAST, err := astDump(out)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range wantAST {
		if i >= len(gotAST) {
			t.Fatalf("AST truncated at node %d (%s)", i, want)
		}
		if gotAST[i] != want {
			t.Fatalf("AST node %d: got %s, want %s", i, gotAST[i], want)
		}
	}
	if len(gotAST) > len(wantAST) {
		t.Fatalf("AST gained node %d (%s)", len(wantAST), gotAST[len(wantAST)])
	}
}

// TestSinglePass holds the pipeline to the claim its shape rests on:
// the rules run once per file and the layout is settled when they
// finish. For every corpus file, one run of the rule sequence over one
// parse must render exactly what formatSrc returns — leaving formatSrc
// no room for a loop — and a second full run over that output must
// return it unchanged.
func TestSinglePass(t *testing.T) {
	files := corpus(t)
	for _, reflow := range []bool{false, true} {
		for _, path := range files {
			name := fmt.Sprintf("%s/reflow=%v", path, reflow)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "x.go", src,
					parser.ParseComments|parser.SkipObjectResolution,
				)
				if err != nil {
					t.Fatalf("does not parse: %v", err)
				}
				if ast.IsGenerated(file) {
					out := formatted(t, path, reflow, false)
					if !bytes.Equal(out, src) {
						t.Errorf("generated file did not pass through:\n%s",
							firstDiff(src, out),
						)
					}
					return
				}
				layout(fset, fset.File(file.Pos()), file, reflow)
				once, err := printNode(fset, file)
				if err != nil {
					t.Fatal(err)
				}
				out := formatted(t, path, reflow, false)
				if once != string(out) {
					t.Errorf("formatSrc ran the rules twice:\n%s",
						firstDiff(out, []byte(once)),
					)
				}
				again := formatted(t, path, reflow, true)
				if !bytes.Equal(again, out) {
					t.Errorf("a second full run changed the output:\n%s",
						firstDiff(out, again),
					)
				}
			})
		}
	}
}

// corpus returns the Go sources every property is checked against: the
// golden inputs, the tools module itself, and a sibling module whose
// layout the formatter is meant to preserve.
func corpus(t *testing.T) (paths []string) {
	t.Helper()
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, inputs...)
	for _, root := range []string{"../..", "../../../strictvar"} {
		if _, err := os.Stat(root); err != nil {
			t.Logf("corpus %s unavailable: %v", root, err)
			continue
		}
		walk := func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, ".go") {
				paths = append(paths, p)
			}
			return nil
		}
		if err := filepath.WalkDir(root, walk); err != nil {
			t.Fatal(err)
		}
	}
	if len(paths) == 0 {
		t.Fatal("empty corpus")
	}
	return paths
}

// comments returns each comment's text with runs of whitespace
// collapsed, so a reflow that only rewraps compares equal.
func comments(src []byte) (out []string, err error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			text := strings.TrimPrefix(c.Text, "//")
			out = append(out, strings.Join(strings.Fields(text), " "))
		}
	}
	return out, nil
}

// astDump returns a position-free, comment-free structural dump of the
// parsed file, one entry per node or field. Positions are skipped so
// legal layout changes — added trailing commas, dropped decl-group
// parens — compare equal while any real structural change shows up.
func astDump(src []byte) (out []string, err error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	dumpNode(&out, "", reflect.ValueOf(file))
	return out, nil
}

var (
	posType = reflect.TypeFor[token.Pos]()
	cgType  = reflect.TypeFor[*ast.CommentGroup]()
)

func dumpNode(out *[]string, path string, v reflect.Value) {
	if !v.IsValid() || v.Type() == posType || v.Type() == cgType {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			*out = append(*out, path+"=nil")
			return
		}
		dumpNode(out, path, v.Elem())
	case reflect.Struct:
		if v.Type().Name() == "Object" || v.Type().Name() == "Scope" {
			return
		}
		*out = append(*out, path+"::"+v.Type().String())
		for i := range v.NumField() {
			f := v.Type().Field(i)
			skip := f.Name == "Comments" || f.Name == "Doc" ||
				f.Name == "Comment" || f.PkgPath != ""
			if skip {
				continue
			}
			dumpNode(out, path+"."+f.Name, v.Field(i))
		}
	case reflect.Slice:
		for i := range v.Len() {
			dumpNode(out, fmt.Sprintf("%s[%d]", path, i), v.Index(i))
		}
	default:
		*out = append(*out, fmt.Sprintf("%s=%v", path, v.Interface()))
	}
}

// firstDiff renders the first line at which a and b disagree, with
// surrounding context.
func firstDiff(a, b []byte) string {
	var (
		la = strings.Split(string(a), "\n")
		lb = strings.Split(string(b), "\n")
	)
	for i := range la {
		if i >= len(lb) {
			return fmt.Sprintf("line %d: got %q, want <eof>", i+1, la[i])
		}
		if la[i] == lb[i] {
			continue
		}
		var ctx strings.Builder
		for j := max(0, i-2); j < min(len(la), i+3); j++ {
			fmt.Fprintf(&ctx, "  got  %d: %q\n", j+1, la[j])
		}
		for j := max(0, i-2); j < min(len(lb), i+3); j++ {
			fmt.Fprintf(&ctx, "  want %d: %q\n", j+1, lb[j])
		}
		return fmt.Sprintf("first diff at line %d:\n%s", i+1, ctx.String())
	}
	if len(lb) > len(la) {
		return fmt.Sprintf(
			"line %d: got <eof>, want %q", len(la)+1, lb[len(la)],
		)
	}
	return ""
}
