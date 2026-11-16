// Package builtinshadow provides an analysis.Analyzer that reports a
// declaration that shadows a predeclared identifier.
//
// Go declares the predeclared identifiers in the universe block
// (https://go.dev/ref/spec#Predeclared_identifiers), and a declaration in an
// inner scope shadows one in an outer scope
// (https://go.dev/ref/spec#Declarations_and_scope). A declaration that reuses
// one of these names makes later references to that name resolve to the
// declaration.
//
//	var len = 3
//
// The README catalogs every check and exemption.
//
// Report only: a rename would have to update every use in the declaration's
// scope, and the declaration may be intentional.
//
// The rule is adapted from revive's redefines-builtin-id.
package builtinshadow

import (
	"go/ast"
	"go/token"
	"go/types"
	"go/version"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "builtinshadow",
	Doc:  "reports a declaration that shadows a predeclared identifier",
	Run:  run,
}

// since records the Go version that added a predeclared identifier. An
// identifier not listed here has been predeclared since Go 1.
var since = map[string]string{
	"any":        "go1.18",
	"comparable": "go1.18",
	"clear":      "go1.21",
	"max":        "go1.21",
	"min":        "go1.21",
}

func run(pass *analysis.Pass) (any, error) {
	versions := make(map[string]string, len(pass.Files))
	for _, file := range pass.Files {
		name := pass.Fset.PositionFor(file.Pos(), false).Filename
		versions[name] = pass.TypesInfo.FileVersions[file]
	}
	noBody, pkgVers := unbound(pass), packageVersion(versions)
	for ident, obj := range pass.TypesInfo.Defs {
		// A field and a method have no parent scope, so neither shadows a
		// name in a lexical block. A label is in a separate namespace.
		if obj == nil || obj.Parent() == nil {
			continue
		}
		if _, ok := obj.(*types.Label); ok {
			continue
		}
		if inSpans(noBody, ident.Pos()) {
			continue
		}
		report(pass, versions, pkgVers, ident)
	}
	for _, file := range pass.Files {
		for _, spec := range file.Imports {
			// An unaliased import's name is implicit, so it is not in Defs.
			if spec.Name != nil {
				continue
			}
			pkg, ok := pass.TypesInfo.Implicits[spec].(*types.PkgName)
			if ok {
				reportImport(pass, versions, spec, pkg)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			// The type checker records a type switch variable on each case
			// clause; the identifier in the guard has no object. The
			// declaration is read from the syntax.
			sw, ok := n.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			assign, ok := sw.Assign.(*ast.AssignStmt)
			if !ok || assign.Tok != token.DEFINE {
				return true
			}
			for _, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					report(pass, versions, pkgVers, ident)
				}
			}
			return true
		})
	}
	return nil, nil
}

// report reports ident when its name is predeclared in its file. A
// package-level declaration is visible to every file in the package, so the
// highest file version decides whether its name is predeclared.
func report(pass *analysis.Pass, versions map[string]string, pkgVersion string, ident *ast.Ident) {
	goVersion := fileVersion(pass, versions, ident.Pos())
	if packageScope(pass, ident) {
		goVersion = pkgVersion
	}
	reportName(pass, goVersion, ident.Pos(), ident.Name)
}

// reportImport reports the name an unaliased import introduces.
func reportImport(pass *analysis.Pass, versions map[string]string, spec *ast.ImportSpec, pkg *types.PkgName) {
	pos := spec.Path.Pos()
	reportName(pass, fileVersion(pass, versions, pos), pos, pkg.Name())
}

// fileVersion returns the language version of the file containing pos.
func fileVersion(pass *analysis.Pass, versions map[string]string, pos token.Pos) string {
	return versions[pass.Fset.PositionFor(pos, false).Filename]
}

// packageScope reports whether ident declares a package-level object.
func packageScope(pass *analysis.Pass, ident *ast.Ident) bool {
	obj := pass.TypesInfo.Defs[ident]
	return obj != nil && obj.Parent() == pass.Pkg.Scope()
}

// reportName reports name at pos when it is predeclared at goVersion.
func reportName(pass *analysis.Pass, goVersion string, pos token.Pos, name string) {
	if !predeclared(name, goVersion) {
		return
	}
	pass.Reportf(
		pos,
		"declaration shadows the predeclared identifier %q", name,
	)
}

// packageVersion returns the highest version among the files, or the empty
// version when any file names none.
func packageVersion(versions map[string]string) (out string) {
	for _, goVersion := range versions {
		if goVersion == "" {
			return ""
		}
		if out == "" || version.Compare(goVersion, out) > 0 {
			out = goVersion
		}
	}
	return out
}

// span is a range of source positions.
type span struct{ start, end token.Pos }

// unbound returns the parameter, result, and receiver names of function types
// and declarations that have no body. Nothing can refer to those names.
func unbound(pass *analysis.Pass) (spans []span) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			typ, ok := n.(*ast.FuncType)
			if ok && !hasBody(stack) {
				spans = appendNames(spans, typ.Params)
				spans = appendNames(spans, typ.Results)
			}
			stack = append(stack, n)
			return true
		})
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Body == nil {
				spans = appendNames(spans, fn.Recv)
			}
		}
	}
	return spans
}

// hasBody reports whether the function type belongs to a declaration or
// literal with a body.
func hasBody(stack []ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	switch parent := stack[len(stack)-1].(type) {
	case *ast.FuncDecl:
		return parent.Body != nil
	case *ast.FuncLit:
		return parent.Body != nil
	}
	return false
}

// appendNames appends a span for each name in fields.
func appendNames(spans []span, fields *ast.FieldList) []span {
	if fields == nil {
		return spans
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			spans = append(spans, span{name.Pos(), name.End()})
		}
	}
	return spans
}

// inSpans reports whether pos is in any span.
func inSpans(spans []span, pos token.Pos) bool {
	for _, s := range spans {
		if pos >= s.start && pos <= s.end {
			return true
		}
	}
	return false
}

// predeclared reports whether name is predeclared in a file at goVersion. An
// empty goVersion means the type checker enforced none, so every name is
// predeclared.
func predeclared(name, goVersion string) bool {
	if types.Universe.Lookup(name) == nil {
		return false
	}
	added, ok := since[name]
	return !ok || goVersion == "" || version.Compare(goVersion, added) >= 0
}
