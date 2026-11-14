// Package redundantalias provides an analysis.Analyzer that reports an import
// alias equal to the declared package name and the last element of its path.
//
// The import declarations section of the spec makes the package's own name the
// default: "If the PackageName is omitted, it defaults to the identifier
// specified in the package clause of the imported package." An alias equal to
// that name is the default written out.
//
// A path like example.com/work/v2 ends in v2 while its package is called task.
// Nothing in the import but the alias says the package is task, so the rule
// leaves task "example.com/work/v2" alone. An alias named v2 for the same path
// names nothing the package declares, and removing it would leave every v2.X
// use unresolved.
//
// The rule is like revive's redundant-import-alias, which compares the alias
// to the last element of the path.
//
// The fix carries a comment written between the alias and the path to the end
// of the spec, so dropping the alias does not lose it.
package redundantalias

import (
	"go/ast"
	"go/types"
	"path"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "redundantalias",
	Doc:  "reports a redundant import alias",
	Run:  run,
}

// checker holds one file of the package.
type checker struct {
	pass *analysis.Pass
	file *ast.File
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		c := checker{pass, file}
		for _, spec := range file.Imports {
			if c.redundant(spec) {
				c.report(spec)
			}
		}
	}
	return nil, nil
}

func (c checker) redundant(spec *ast.ImportSpec) bool {
	name := spec.Name
	if name == nil {
		return false
	}
	obj, ok := c.pass.TypesInfo.Defs[name].(*types.PkgName)
	if !ok {
		return false
	}
	imported := obj.Imported()
	return imported.Name() == name.Name &&
		path.Base(imported.Path()) == name.Name
}

func (c checker) report(spec *ast.ImportSpec) {
	c.pass.Report(analysis.Diagnostic{
		Pos:     spec.Name.Pos(),
		End:     spec.Name.End(),
		Message: "redundant import alias " + strconv.Quote(spec.Name.Name),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "Drop the alias",
			TextEdits: []analysis.TextEdit{c.unalias(spec)},
		}},
	})
}

// unalias returns the edit that drops the alias, carrying any comment written
// between it and the path to the end of the spec.
func (c checker) unalias(spec *ast.ImportSpec) analysis.TextEdit {
	comments := c.between(spec)
	if len(comments) == 0 {
		return analysis.TextEdit{Pos: spec.Name.Pos(), End: spec.Path.Pos()}
	}
	return analysis.TextEdit{
		Pos:     spec.Name.Pos(),
		End:     spec.Path.End(),
		NewText: []byte(spec.Path.Value + " " + strings.Join(comments, " ")),
	}
}

// between returns the text of every comment written between the alias and the
// path.
func (c checker) between(spec *ast.ImportSpec) (out []string) {
	for _, group := range c.file.Comments {
		for _, comment := range group.List {
			if inside(comment, spec) {
				out = append(out, comment.Text)
			}
		}
	}
	return out
}

// inside reports whether comment is written between the alias and the path.
func inside(comment *ast.Comment, spec *ast.ImportSpec) bool {
	return comment.Pos() >= spec.Name.End() && comment.End() <= spec.Path.Pos()
}
