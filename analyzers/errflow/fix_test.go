package errflow

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// TestFixText holds each fix to the text it writes before a driver reformats
// it. analysistest formats a fixed file before comparing it with the golden,
// so an indent the fix writes twice, or one it fails to write, goes unseen
// there. A fix begins at the statement it replaces, so its first line carries
// no indentation and every later line carries its own.
func TestFixText(t *testing.T) {
	tests := []struct{ name, src, want string }{{
		name: "expansion",
		src: `package p

func load() error { return nil }

func save() error { return nil }

func f() error {
	if err := load(); err != nil {
		return err
	}
	return save()
}
`,
		want: "if err := save(); err != nil {\n\t\treturn err\n\t}\n\treturn nil",
	}, {
		name: "collapse keeps the wrap",
		src: `package p

func rename(a, b, c, d, e, f string) error { return nil }

func f() error {
	if err := rename(
		"alpha-first",
		"beta-second",
		"gamma-third",
		"delta-fourth",
		"epsilon-fifth",
		"zeta-sixth",
	); err != nil {
		return err
	}
	return nil
}
`,
		want: "return rename(\n\t\t\"alpha-first\",\n\t\t\"beta-second\",\n" +
			"\t\t\"gamma-third\",\n\t\t\"delta-fourth\",\n\t\t\"epsilon-fifth\",\n" +
			"\t\t\"zeta-sixth\",\n\t)",
	}, {
		name: "collapse carries a comment above the return",
		src: `package p

func load() error { return nil }

func f() error {
	if err := load(); err != nil {
		// the load can fail
		return err
	}
	return nil
}
`,
		want: "// the load can fail\n\treturn load()",
	}, {
		name: "collapse keeps a trailing comment above the return",
		src: `package p

func load() error { return nil }

func f() error {
	if err := load(); err != nil { // the load can fail
		return err
	}
	return nil
}
`,
		want: "// the load can fail\n\treturn load()",
	}, {
		name: "collapse carries a block comment with its shape",
		src: `package p

func load() error { return nil }

func f() error {
	if err := load(); err != nil {
		/*
			The load failed:
			  - the file may be missing
		*/
		return err
	}
	return nil
}
`,
		want: "/*\n\t\tThe load failed:\n\t\t  - the file may be missing\n" +
			"\t*/\n\treturn load()",
	}, {
		name: "expansion keeps a directive the call holds",
		src: `package p

func load() error { return nil }

func save() error { return nil }

func f() error {
	if err := load(); err != nil {
		return err
	}
	return save( /*line alpha.go:300*/ )
}
`,
		want: "if err := save( /*line alpha.go:300*/ ); err != nil {\n" +
			"\t\treturn err\n\t}\n\treturn nil",
	}, {
		name: "expansion carries a trailing comment above the guard",
		src: `package p

func load() error { return nil }

func save() error { return nil }

func f() error {
	if err := load(); err != nil {
		return err
	}
	return save() // the caller reports it
}
`,
		want: "// the caller reports it\n\tif err := save(); err != nil {\n" +
			"\t\treturn err\n\t}\n\treturn nil",
	}, {
		name: "collapse keeps a broken call that gofmt closes",
		src: `package p

func load() error { return nil }

func f() error {
	if err := load(
	); err != nil {
		return err
	}
	return nil
}
`,
		want: "return load(\n\t)",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fixText(t, test.src); got != test.want {
				t.Fatalf("fix wrote\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

// TestLineDirective holds both fixes to the bytes they replace. A line table
// reports the line a directive names, so a fix that asks the table for the end
// of an edited line reads a line somewhere else, where a comment it never saw
// sits: the collapse withholds a fix it could write, and the expansion
// withholds one too.
func TestLineDirective(t *testing.T) {
	const header = `package p

func load() error { return nil }

func save() error { return nil }

//line alpha.go:100
`
	tests := []struct{ name, src, want string }{{
		name: "collapse",
		src: header + `
func f() error {
	if err := load(); err != nil { // the load failed
		return err
	}
	return nil
}

// untouched
func g() error {
	return nil
}
`,
		want: header + `
func f() error {
	// the load failed
	return load()
}

// untouched
func g() error {
	return nil
}
`,
	}, {
		name: "expansion",
		src: header + `
func f() error {
	if err := load(); err != nil {
		return err
	}
	return save() // written above the guard
}

// untouched
func g() error {
	return nil
}
`,
		want: header + `
func f() error {
	if err := load(); err != nil {
		return err
	}
	// written above the guard
	if err := save(); err != nil {
		return err
	}
	return nil
}

// untouched
func g() error {
	return nil
}
`,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := applied(t, test.src); got != test.want {
				t.Fatalf("fix wrote\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

// TestTrailingCode holds the expansion to withholding its fix when the
// return's line holds code, because the fix deletes the text around the
// comment it moves. A golden cannot say this: one that matches the formatted
// input passes whether no fix is offered or the fix rewrites the same bytes.
func TestTrailingCode(t *testing.T) {
	const header = `package p

func load() error { return nil }

func save() error { return nil }

func f() error {
	if err := load(); err != nil {
		return err
	}
`
	tests := []struct{ name, tail string }{
		{"code before the comment", "return save() } // the caller reports it"},
		{"code after the comment", "return save() /* the caller reports it */ }"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, got := analyze(t, header+test.tail+"\n")
			if len(got) != 1 {
				t.Fatalf("found %d diagnostics, want one", len(got))
			}
			if fixes := got[0].SuggestedFixes; len(fixes) != 0 {
				t.Fatalf("found %d fixes, want none", len(fixes))
			}
		})
	}
}

// TestEmptyBody holds the analyzer to a function whose body holds no
// statements. The package does not type-check, and a driver still runs the
// suite over it.
func TestEmptyBody(t *testing.T) {
	_, got := analyze(t, "package p\n\nfunc f() error {}\n")
	if len(got) != 0 {
		t.Fatalf("found %d diagnostics, want none", len(got))
	}
}

// fixText returns the text of the analyzer's only suggested fix, as the
// analyzer writes it.
func fixText(t *testing.T, src string) string {
	t.Helper()
	_, edit := onlyEdit(t, src)
	return string(edit.NewText)
}

// onlyEdit returns the file set and the edit of the analyzer's only fix.
func onlyEdit(t *testing.T, src string) (*token.FileSet, analysis.TextEdit) {
	t.Helper()
	fset, got := analyze(t, src)
	if len(got) != 1 || len(got[0].SuggestedFixes) != 1 {
		t.Fatalf("found %d diagnostics, want one with a fix", len(got))
	}
	return fset, got[0].SuggestedFixes[0].TextEdits[0]
}

// analyze returns the file set and the diagnostics src produces. A package
// that does not type-check still reaches the analyzer, so the check's error is
// not a reason to stop.
func analyze(t *testing.T, src string) (*token.FileSet, []analysis.Diagnostic) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	pkg, _ := new(types.Config).Check("p", fset, []*ast.File{file}, info)
	var found []analysis.Diagnostic
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     []*ast.File{file},
		Pkg:       pkg,
		TypesInfo: info,
		ReadFile:  func(string) ([]byte, error) { return []byte(src), nil },
		Report:    func(d analysis.Diagnostic) { found = append(found, d) },
	}
	if _, err := Analyzer.Run(pass); err != nil {
		t.Fatal(err)
	}
	return fset, found
}

// applied returns src with the analyzer's only fix applied, unformatted.
func applied(t *testing.T, src string) string {
	t.Helper()
	var (
		fset, edit = onlyEdit(t, src)
		file       = fset.File(edit.Pos)
	)
	return src[:file.Offset(edit.Pos)] + string(edit.NewText) +
		src[file.Offset(edit.End):]
}
