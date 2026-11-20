package selectsleep

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// TestFixText holds each fix to the text it writes before a driver reformats
// it. analysistest formats a fixed file before comparing it with the golden,
// so an indent the fix writes twice, or one it fails to write, goes unseen
// there.
func TestFixText(t *testing.T) {
	const header = `package p

import "time"

func use(...any) {}

`
	tests := []struct{ name, body, want string }{{
		name: "empty case",
		body: `func wait() {
	select {
	case <-time.After(time.Second):
	}
}
`,
		want: "time.Sleep(time.Second)",
	}, {
		name: "body follows the call",
		body: `func wait() {
	select {
	case <-time.After(time.Second):
		use()
		use()
	}
}
`,
		want: "\ttime.Sleep(time.Second)\n\tuse()\n\tuse()\n",
	}, {
		name: "body starts on the case line",
		body: `func wait() {
	select {
	case <-time.After(time.Second): use()
	}
}
`,
		want: "\ttime.Sleep(time.Second)\n\tuse()\n",
	}, {
		name: "body in a block",
		body: `func wait() {
	select {
	case <-time.After(time.Second):
		for range 2 {
			use()
		}
	}
}
`,
		want: "\ttime.Sleep(time.Second)\n\tfor range 2 {\n\t\tuse()\n\t}\n",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fixText(t, header+test.body); got != test.want {
				t.Fatalf("fix wrote\n%q\nwant\n%q", got, test.want)
			}
		})
	}
}

// fixText returns the text of the analyzer's only suggested fix, as the
// analyzer writes it.
func fixText(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:  map[ast.Expr]types.TypeAndValue{},
		Defs:   map[*ast.Ident]types.Object{},
		Uses:   map[*ast.Ident]types.Object{},
		Scopes: map[ast.Node]*types.Scope{},
	}
	cfg := &types.Config{Importer: importer.Default()}
	pkg, err := cfg.Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
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
	if len(found) != 1 || len(found[0].SuggestedFixes) != 1 {
		t.Fatalf("found %d diagnostics, want one with a fix", len(found))
	}
	return string(found[0].SuggestedFixes[0].TextEdits[0].NewText)
}
