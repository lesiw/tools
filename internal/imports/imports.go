// Package imports holds what an analyzer needs to write a package qualifier: a
// file's imports by path, the name one of them reaches at a position, and the
// declaration an edit adds when the import is missing.
package imports

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Names maps the path of each import in file to the name the file binds it to.
func Names(pass *analysis.Pass, file *ast.File) map[string]string {
	names := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		obj := pass.TypesInfo.Defs[spec.Name]
		if obj == nil {
			obj = pass.TypesInfo.Implicits[spec]
		}
		if pkgName, ok := obj.(*types.PkgName); ok {
			names[pkgName.Imported().Path()] = pkgName.Name()
		}
	}
	return names
}

// Qualifier returns the name to write for the package imported at importPath,
// and reports whether the name reaches that package at pos.
func Qualifier(pass *analysis.Pass, names map[string]string, importPath string, pos token.Pos) (string, bool) {
	name, ok := names[importPath]
	if ok && (name == "." || name == "_") {
		return path.Base(importPath), false
	}
	if !ok {
		name = path.Base(importPath)
	}
	// A declaration or another import can hold the name the rewrite writes.
	if scope := pass.Pkg.Scope().Innermost(pos); scope != nil {
		if _, obj := scope.LookupParent(name, pos); obj != nil {
			pkgName, ok := obj.(*types.PkgName)
			if !ok || pkgName.Imported().Path() != importPath {
				return name, false
			}
		}
	}
	return name, true
}

// Add returns the edit that adds paths as an import declaration after the
// package clause. A driver that only formats leaves the declaration in place,
// and goimports folds it into the file's own block.
func Add(pass *analysis.Pass, file *ast.File, paths ...string) analysis.TextEdit {
	var (
		tokFile = pass.Fset.File(file.Pos())
		b       strings.Builder
	)
	b.WriteString("\n\nimport (\n")
	for _, importPath := range paths {
		b.WriteString("\t" + strconv.Quote(importPath) + "\n")
	}
	b.WriteString(")")
	pos := tokFile.LineStart(tokFile.Line(file.Name.Pos())+1) - 1
	return analysis.TextEdit{Pos: pos, End: pos, NewText: []byte(b.String())}
}
