// Package importgroup provides an analysis.Analyzer that reports an import
// block that is not grouped standard library, other modules, then the packages
// the -local flag names.
//
// A block has three groups, in this order: the standard library, other
// modules, and the packages the -local flag names. One blank line separates
// the groups, and each group is sorted by path. goimports sorts within a group
// and separates the standard library from the rest, so a block can be grouped
// in many ways and still compile and format.
package importgroup

import (
	"cmp"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// local holds the prefixes whose packages take the last group, as -local gives
// them to goimports: a comma-separated list, all of them one group.
var local string

var Analyzer = &analysis.Analyzer{
	Name: "importgroup",
	Doc:  "reports an import block that is not grouped standard, other, local",
	Run:  run,
}

func init() {
	Analyzer.Flags.StringVar(
		&local, "local", "",
		"comma-separated prefixes of the last group",
	)
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.IMPORT || !gen.Lparen.IsValid() {
				continue
			}
			if len(gen.Specs) < 2 {
				continue
			}
			check(pass, file, gen)
		}
	}
	return nil, nil
}

// entry is one import of a block: the path it names, and the text the file
// writes for it, comments included.
type entry struct {
	path string
	text string
}

// check reports the block whose entries are not in canonical order.
func check(pass *analysis.Pass, file *ast.File, gen *ast.GenDecl) {
	src, ok := read(pass, gen)
	if !ok {
		return
	}
	entries, ok := entries(pass, gen, src)
	if !ok {
		return
	}
	// The rewrite writes each spec's own text, so a comment that belongs to
	// no spec would be dropped.
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if c.Pos() > gen.Pos() && c.End() <= gen.End() && !owned(gen, c) {
				return
			}
		}
	}
	want := render(classify(entries))
	tokFile := pass.Fset.File(gen.Pos())
	have := string(src[tokFile.Offset(gen.Pos()):tokFile.Offset(gen.End())])
	if have == want {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     gen.Specs[0].Pos(),
		End:     gen.End(),
		Message: "import block is not grouped standard, other, local",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Rewrite the import block",
			TextEdits: []analysis.TextEdit{{
				Pos:     gen.Pos(),
				End:     gen.End(),
				NewText: []byte(want),
			}},
		}},
	})
}

// entries returns the block's imports, each with the text the file writes for
// it.
func entries(pass *analysis.Pass, gen *ast.GenDecl, src []byte) (out []entry, ok bool) {
	tokFile := pass.Fset.File(gen.Pos())
	for _, s := range gen.Specs {
		imp, isImport := s.(*ast.ImportSpec)
		if !isImport {
			return nil, false
		}
		text, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, false // a block this rule cannot read is left alone
		}
		lo := imp.Pos()
		if imp.Doc != nil {
			lo = imp.Doc.Pos()
		}
		hi := imp.End()
		if imp.Comment != nil {
			hi = imp.Comment.End()
		}
		out = append(out, entry{
			path: text,
			text: string(src[tokFile.Offset(lo):tokFile.Offset(hi)]),
		})
	}
	return out, true
}

// owned reports whether the comment belongs to a spec, which carries it when
// the block is rewritten.
func owned(gen *ast.GenDecl, c *ast.Comment) bool {
	for _, s := range gen.Specs {
		imp, ok := s.(*ast.ImportSpec)
		if !ok {
			return false
		}
		for _, g := range []*ast.CommentGroup{imp.Doc, imp.Comment} {
			if g != nil && c.Pos() >= g.Pos() && c.End() <= g.End() {
				return true
			}
		}
		if c.Pos() >= imp.Pos() && c.End() <= imp.End() {
			return true
		}
	}
	return false
}

// read returns the source of the file that holds gen.
func read(pass *analysis.Pass, gen *ast.GenDecl) ([]byte, bool) {
	tokFile := pass.Fset.File(gen.Pos())
	if tokFile == nil {
		return nil, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return nil, false
	}
	return src, true
}

// classify returns the entries in the groups goimports gives them: the
// standard library, then other modules, then the packages the -local flag
// names.
func classify(entries []entry) [][]entry {
	var (
		prefixes = locals()
		groups   = make([][]entry, 3)
	)
	for _, e := range entries {
		switch {
		case standard(e.path):
			groups[0] = append(groups[0], e)
		case !prefix(e.path, prefixes):
			groups[1] = append(groups[1], e)
		default:
			groups[2] = append(groups[2], e)
		}
	}
	for _, g := range groups {
		slices.SortFunc(g, func(a, b entry) int {
			return cmp.Compare(a.path, b.path)
		})
	}
	return groups
}

// locals returns the prefixes the flag names, in order.
func locals() (out []string) {
	for p := range strings.SplitSeq(local, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// prefix reports whether p falls under one of the prefixes.
func prefix(p string, prefixes []string) bool {
	for _, want := range prefixes {
		if under(p, want) {
			return true
		}
	}
	return false
}

// standard reports whether p names a standard library package, whose first
// element holds no dot.
func standard(p string) bool {
	first, _, _ := strings.Cut(p, "/")
	return !strings.Contains(first, ".")
}

// under reports whether p is prefix or a package in it.
func under(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

// isComment reports whether the text of an entry begins with a comment.
func isComment(text string) bool {
	return strings.HasPrefix(text, "//") || strings.HasPrefix(text, "/*")
}

// indent writes text with its first line at one tab, which is where a spec
// sits in a block. Later lines keep the indentation the source gave them, so a
// block comment keeps its interior.
func indent(text string) string {
	first, rest, ok := strings.Cut(text, "\n")
	if !ok {
		return "\t" + strings.TrimLeft(first, " \t")
	}
	return "\t" + strings.TrimLeft(first, " \t") + "\n" + rest
}

// render writes the block as the rule wants it.
func render(groups [][]entry) string {
	var b strings.Builder
	b.WriteString("import (\n")
	first := true
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		for i, e := range g {
			// goimports starts a new section at a comment, so a spec
			// with a doc comment keeps the blank line before it. A
			// render that dropped the line would rewrite the block
			// on every round.
			if i > 0 && isComment(e.text) {
				b.WriteString("\n")
			}
			b.WriteString(indent(e.text))
			b.WriteString("\n")
		}
	}
	b.WriteString(")")
	return b.String()
}
