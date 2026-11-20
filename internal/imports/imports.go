// Package imports holds what an analyzer needs to write a package qualifier: a
// file's imports by path, the name one of them reaches at a position, and the
// declaration an edit adds when the file does not import the package.
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
	if obj := Lookup(pass, name, pos); obj != nil {
		pkgName, ok := obj.(*types.PkgName)
		if !ok || pkgName.Imported().Path() != importPath {
			return name, false
		}
	}
	return name, true
}

// Lookup returns the object that name reaches at pos, or nil when the position
// is outside the package's scopes or no declaration reaches it.
func Lookup(pass *analysis.Pass, name string, pos token.Pos) types.Object {
	scope := pass.Pkg.Scope().Innermost(pos)
	if scope == nil {
		return nil
	}
	_, obj := scope.LookupParent(name, pos)
	return obj
}

// Add returns the edit that adds paths as an import declaration after the
// package clause. One path is written as a single declaration, since nothing
// unwraps a block of one.
func Add(pass *analysis.Pass, file *ast.File, paths ...string) analysis.TextEdit {
	var b strings.Builder
	if len(paths) == 1 {
		b.WriteString("import " + strconv.Quote(paths[0]))
	} else {
		b.WriteString("import (\n")
		for _, importPath := range paths {
			b.WriteString("\t" + strconv.Quote(importPath) + "\n")
		}
		b.WriteString(")")
	}
	return afterPackage(pass, file, b.String())
}

// AddBlank returns the edit that adds a blank import of importPath after the
// package clause. A file that needs a package but never names it takes the
// blank identifier.
func AddBlank(pass *analysis.Pass, file *ast.File, importPath string) analysis.TextEdit {
	return afterPackage(pass, file, "import _ "+strconv.Quote(importPath))
}

// afterPackage returns the edit that writes text after the package clause. A
// driver that only formats leaves the declaration in place, and goimports
// folds it into the file's own block. A file can end at its package clause,
// which has no line after it.
func afterPackage(pass *analysis.Pass, file *ast.File, text string) analysis.TextEdit {
	var (
		tokFile = pass.Fset.File(file.Pos())
		pos     = file.Name.End()
	)
	if tokFile.LineCount() > tokFile.Line(file.Name.Pos()) {
		pos = tokFile.LineStart(tokFile.Line(file.Name.Pos())+1) - 1
	}
	return analysis.TextEdit{
		Pos:     pos,
		End:     pos,
		NewText: []byte("\n\n" + text),
	}
}
