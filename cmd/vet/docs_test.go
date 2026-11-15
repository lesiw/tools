package main

import (
	"flag"
	"go/doc/comment"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"lesiw.io/tools/internal/suite"
)

var update = flag.Bool(
	"update", false,
	"rewrite cmd/vet/README.md from the doc comment",
)

const readme = "README.md"

// docComment returns the package comment of main.go.
func docComment(t *testing.T) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if f.Doc == nil {
		t.Fatal("main.go has no package comment")
	}
	return f.Doc.Text()
}

// markdown renders a package comment as a README: the title, then every block
// with its headings demoted one level and its code blocks fenced.
func markdown(doc string) string {
	var (
		parse   comment.Parser
		printer = comment.Printer{
			HeadingLevel: 2,
			HeadingID:    func(*comment.Heading) string { return "" },
		}
		out strings.Builder
	)
	out.WriteString("# cmd/vet\n\n")
	for _, block := range parse.Parse(doc).Content {
		if code, ok := block.(*comment.Code); ok {
			out.WriteString("```\n")
			out.WriteString(indent(code.Text))
			out.WriteString("```\n\n")
			continue
		}
		out.Write(printer.Markdown(&comment.Doc{
			Content: []comment.Block{block},
		}))
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// indent writes each line of code with four spaces where its leading tabs
// were, because Markdown renders a tab as eight columns.
func indent(code string) string {
	var out strings.Builder
	for line := range strings.SplitSeq(strings.TrimSuffix(code, "\n"), "\n") {
		n := len(line) - len(strings.TrimLeft(line, "\t"))
		out.WriteString(strings.Repeat("    ", n))
		out.WriteString(line[n:])
		out.WriteString("\n")
	}
	return out.String()
}

func TestREADMEUpToDate(t *testing.T) {
	want := markdown(docComment(t))
	if *update {
		if err := os.WriteFile(readme, []byte(want), 0o666); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("%v (run go test -update)", err)
	}
	if string(got) != want {
		t.Fatal("README.md is stale; run go test -update")
	}
}

// section is one heading of the doc comment and the blocks below it.
type section struct {
	name   string
	blocks []comment.Block
}

// sections returns the doc comment's sections, in the order they are written.
func sections(doc string) (out []section) {
	var parse comment.Parser
	for _, block := range parse.Parse(doc).Content {
		if heading, ok := block.(*comment.Heading); ok {
			out = append(out, section{name: headingName(heading)})
			continue
		}
		if len(out) == 0 {
			continue // the introduction, above the first heading
		}
		out[len(out)-1].blocks = append(out[len(out)-1].blocks, block)
	}
	return out
}

// headingName returns the words of a heading.
func headingName(h *comment.Heading) string {
	var name strings.Builder
	for _, text := range h.Text {
		if plain, ok := text.(comment.Plain); ok {
			name.WriteString(string(plain))
		}
	}
	return name.String()
}

// TestDocsAreSorted holds the sections to ascending order, so a reader can
// find an analyzer without reading the page.
func TestDocsAreSorted(t *testing.T) {
	var names []string
	for _, s := range sections(docComment(t)) {
		names = append(names, s.name)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("%s: follows %s", names[i], names[i-1])
		}
	}
}

// TestDocsCoverTheSuite holds the doc to the suite: every analyzer has a
// section, no section names an analyzer the suite does not have, and every
// section carries a description and an example.
func TestDocsCoverTheSuite(t *testing.T) {
	known := map[string]struct{}{}
	for _, a := range suite.Analyzers() {
		known[a.Name] = struct{}{}
	}
	documented := map[string]struct{}{}
	for _, s := range sections(docComment(t)) {
		documented[s.name] = struct{}{}
		if _, ok := known[s.name]; !ok {
			t.Errorf("%s: documented but not in the suite", s.name)
		}
		var described, exampled bool
		for _, block := range s.blocks {
			switch block.(type) {
			case *comment.Paragraph:
				described = true
			case *comment.Code:
				exampled = true
			}
		}
		if !described {
			t.Errorf("%s: section has no description", s.name)
		}
		if !exampled {
			t.Errorf("%s: section has no example", s.name)
		}
	}
	for _, a := range suite.Analyzers() {
		if _, ok := documented[a.Name]; !ok {
			t.Errorf("%s: no section in the doc comment", a.Name)
		}
	}
}
