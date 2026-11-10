// Package importshadow provides an analysis.Analyzer that reports a
// declaration that shadows an imported package name.
//
// A package name is used as a qualifier throughout the file, so a local that
// reuses it makes every later reference ambiguous until the reader notices
// that the qualifier changed meaning. Renaming the local costs nothing.
package importshadow

import (
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "importshadow",
	Doc:  "reports a declaration that shadows an imported package name",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	imported := imports(pass)
	for ident, obj := range pass.TypesInfo.Defs {
		if obj == nil || ident.Name == "_" {
			continue
		}
		// An import is not shadowed by itself.
		if _, ok := obj.(*types.PkgName); ok {
			continue
		}
		// A struct field and an interface method have no parent scope,
		// so neither shadows anything.
		if obj.Parent() == nil {
			continue
		}
		file := pass.Fset.Position(ident.Pos()).Filename
		if _, ok := imported[file][ident.Name]; !ok {
			continue
		}
		pass.Reportf(ident.Pos(), "declaration shadows the imported package %q", ident.Name)
	}
	return nil, nil
}

// imports returns the names each file's imports introduce, keyed by filename.
func imports(pass *analysis.Pass) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{})
	for _, file := range pass.Files {
		names := make(map[string]struct{})
		for _, spec := range file.Imports {
			if spec.Name != nil {
				// A blank or dot import introduces no name.
				if spec.Name.Name != "_" && spec.Name.Name != "." {
					names[spec.Name.Name] = struct{}{}
				}
				continue
			}
			obj, ok := pass.TypesInfo.Implicits[spec].(*types.PkgName)
			if !ok {
				continue
			}
			names[obj.Name()] = struct{}{}
		}
		out[pass.Fset.Position(file.Pos()).Filename] = names
	}
	return out
}
