// Package deprecated provides an analysis.Analyzer that reports a use of a
// deprecated identifier or package.
//
// Vendored from gopls' deprecated (gopls/internal/analysis/deprecated,
// golang.org/x/tools/gopls v0.24.0), which is internal and cannot be
// imported. The rule is upstream's; the code is adapted to this repository's
// helpers. This rule also reports a field reached through a generic
// instantiation.
//
// Report only: the substitute for a deprecated symbol is the author's choice,
// and a tool cannot make it. The README catalogs every check, exemption, and
// limit.
package deprecated

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name:      "deprecated",
	Doc:       "reports a use of a deprecated identifier or package",
	Run:       run,
	FactTypes: []analysis.Fact{(*note)(nil)},
}

// marker begins the paragraph that marks a deprecation.
const marker = "Deprecated: "

// note is the deprecation a declaration leaves for the packages that use it.
type note string

func (*note) AFact() {}

func (n *note) String() string { return marker + string(*n) }

// deprecations holds the notes a package can see, by object and by imported
// package.
type deprecations struct {
	objects  map[types.Object]*note
	packages map[*types.Package]*note
}

func run(pass *analysis.Pass) (any, error) {
	seen := collect(pass)
	if len(seen.objects) == 0 && len(seen.packages) == 0 {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				useObject(pass, seen, n)
			case *ast.ImportSpec:
				usePackage(pass, seen, n)
			}
			return true
		})
	}
	return nil, nil
}

// collect exports the deprecations this package declares, and returns the
// ones it can see: its own, and every fact the driver imported for it.
func collect(pass *analysis.Pass) deprecations {
	for _, file := range pass.Files {
		if msg := noteOf(file.Doc); msg != "" {
			pass.ExportPackageFact(&msg)
			break
		}
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool { mark(pass, n); return true })
	}
	out := deprecations{
		objects:  make(map[types.Object]*note),
		packages: make(map[*types.Package]*note),
	}
	for _, fact := range pass.AllObjectFacts() {
		out.objects[fact.Object] = fact.Fact.(*note)
	}
	for _, fact := range pass.AllPackageFacts() {
		out.packages[fact.Package] = fact.Fact.(*note)
	}
	return out
}

// mark exports the note a declaration carries to every name it declares. A
// group's doc reaches all of its names; a spec's own doc overrides it,
// because the walk reaches the spec after the group.
func mark(pass *analysis.Pass, n ast.Node) {
	switch n := n.(type) {
	case *ast.GenDecl:
		switch n.Tok {
		case token.CONST, token.TYPE, token.VAR:
			export(pass, declared(n), n.Doc)
		}
	case *ast.FuncDecl:
		export(pass, []*ast.Ident{n.Name}, n.Doc)
	case *ast.TypeSpec:
		export(pass, []*ast.Ident{n.Name}, n.Doc)
	case *ast.ValueSpec:
		export(pass, n.Names, n.Doc)
	case *ast.StructType:
		fields(pass, n.Fields)
	case *ast.InterfaceType:
		fields(pass, n.Methods)
	}
}

// declared returns the names a type, const or var group declares.
func declared(decl *ast.GenDecl) (names []*ast.Ident) {
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			names = append(names, spec.Name)
		case *ast.ValueSpec:
			names = append(names, spec.Names...)
		}
	}
	return names
}

// fields exports the deprecation of every field or method that carries one.
func fields(pass *analysis.Pass, list *ast.FieldList) {
	for _, field := range list.List {
		export(pass, field.Names, field.Doc, field.Comment)
	}
}

// export records the first note among docs on every name.
func export(pass *analysis.Pass, names []*ast.Ident, docs ...*ast.CommentGroup) {
	for _, doc := range docs {
		msg := noteOf(doc)
		if msg == "" {
			continue
		}
		for _, name := range names {
			if obj := pass.TypesInfo.ObjectOf(name); obj != nil {
				pass.ExportObjectFact(obj, &msg)
			}
		}
		return
	}
}

// noteOf returns the paragraph that begins with the marker, or the rest of a
// one-line comment that holds the marker after other prose. A note written
// across lines is joined into one line.
func noteOf(doc *ast.CommentGroup) note {
	if doc == nil {
		return ""
	}
	text := doc.Text()
	for paragraph := range strings.SplitSeq(text, "\n\n") {
		if msg, ok := strings.CutPrefix(paragraph, marker); ok {
			return note(strings.Join(strings.Fields(msg), " "))
		}
	}
	if len(doc.List) != 1 || !strings.HasPrefix(doc.List[0].Text, "//") {
		return ""
	}
	if _, rest, ok := strings.Cut(text, marker); ok {
		return note(strings.Join(strings.Fields(rest), " "))
	}
	return ""
}

// useObject reports a use of a deprecated object.
func useObject(pass *analysis.Pass, seen deprecations, sel *ast.SelectorExpr) {
	obj := pass.TypesInfo.ObjectOf(sel.Sel)
	// A field or method of an instantiated generic type is not the object the
	// declaration marked.
	if fn, ok := obj.(*types.Func); ok {
		obj = fn.Origin()
	} else if v, ok := obj.(*types.Var); ok {
		obj = v.Origin()
	}
	if obj == nil || obj.Pkg() == nil {
		return
	}
	if exempt(pass.Pkg.Path(), obj.Pkg().Path()) {
		return
	}
	if d, ok := seen.objects[obj]; ok {
		report(pass, d, sel)
	}
}

// usePackage reports an import of a deprecated package.
func usePackage(pass *analysis.Pass, seen deprecations, spec *ast.ImportSpec) {
	obj := pass.TypesInfo.Implicits[spec]
	if spec.Name != nil {
		obj = pass.TypesInfo.ObjectOf(spec.Name)
	}
	pkg, ok := obj.(*types.PkgName)
	if !ok {
		return
	}
	if exempt(pass.Pkg.Path(), pkg.Imported().Path()) {
		return
	}
	if d, ok := seen.packages[pkg.Imported()]; ok {
		report(pass, d, spec.Path)
	}
}

// exempt reports whether the package at path pkg may use the package at path
// other. A package may use its own objects, an external test package may use
// the package it tests, and the main of a test binary may use both.
func exempt(pkg, other string) bool {
	if strings.TrimSuffix(pkg, "_test") == other {
		return true
	}
	generated := strings.TrimSuffix(pkg, ".test")
	return generated == other ||
		generated == strings.TrimSuffix(other, "_test")
}

func report(pass *analysis.Pass, d *note, expr ast.Expr) {
	pass.Report(analysis.Diagnostic{
		Pos:     expr.Pos(),
		End:     expr.End(),
		Message: astexpr.Text(expr) + " is deprecated: " + string(*d),
	})
}
