// Package sortnosort provides an analysis.Analyzer that reports an assignment
// that converts a slice to a sort slice type and assigns it back.
//
// The sort package declares StringSlice, IntSlice, and Float64Slice as types.
// The StringSlice doc says it "attaches the methods of Interface to []string,
// sorting in increasing order." (https://pkg.go.dev/sort#StringSlice). The
// conversion yields a value of that type, and the assignment stores it back in
// the target. Nothing calls the methods, so x = sort.StringSlice(x) sorts
// nothing.
//
// The fix writes a call to the matching function, qualified by the name the
// conversion uses for sort or another name the file imports sort under, or
// written with the bare function name when the file imports sort with a dot
// and that name reaches the imported function. It adds the import when the
// file has no usable import of sort and the package can import it; the README
// lists the cases where the fix is withheld. The call sorts the slice, which
// the assignment did not do; the suggested fix message says that the edit
// changes behavior. The fix is withheld when a comment overlaps the
// assignment, among other cases. The analyzer leaves an assignment alone when
// the target already has one of the sort slice types. The README catalogs
// every check, exemption, and fix.
//
// The rule is adapted from staticcheck's SA4029, which reports the same three
// conversions.
package sortnosort

import (
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"strconv"
	"sync"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "sortnosort",
	Doc:  "reports an assignment that converts a slice to a sort slice type and assigns it back",
	Run:  run,
}

// sorted maps a sort slice type to the name of the function that sorts its
// elements.
var sorted = map[string]string{
	"Float64Slice": "Float64s",
	"IntSlice":     "Ints",
	"StringSlice":  "Strings",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			names = usableNames(pass, file, imports.Names(pass, file))
			dots  = dotPaths(file)
			sites []site
		)
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			if s, ok := match(pass, assign); ok {
				sites = append(sites, s)
			}
			return true
		})
		for i := range sites {
			sites[i].text, sites[i].fixable = replacement(
				pass, file, names, dots, sites, sites[i].call,
				sites[i].assign, sites[i].target, sites[i].sortFunc,
			)
		}
		for _, s := range sites {
			pass.Report(diagnose(pass, file, names, dots, s, sites))
		}
	}
	return nil, nil
}

// site is one assignment the rule reports.
type site struct {
	assign   *ast.AssignStmt
	target   ast.Expr
	call     *ast.CallExpr
	sortFunc string
	typeName string
	text     string
	fixable  bool
}

// match returns the assignment as a site when it converts its target back to a
// sort slice type, and reports false for any other assignment.
func match(pass *analysis.Pass, assign *ast.AssignStmt) (site, bool) {
	if assign.Tok != token.ASSIGN {
		return site{}, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return site{}, false
	}
	var (
		target   = ast.Unparen(assign.Lhs[0])
		call, ok = ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	)
	if !ok || len(call.Args) != 1 {
		return site{}, false
	}
	sortFunc, typeName, ok := conversion(pass, call)
	if !ok || !same(pass, target, call.Args[0]) {
		return site{}, false
	}
	if !sliceType(pass.TypesInfo.TypeOf(target)) {
		return site{}, false
	}
	return site{
		assign:   assign,
		target:   target,
		call:     call,
		sortFunc: sortFunc,
		typeName: typeName,
	}, true
}

// diagnose returns the diagnostic for one site. sites is every assignment the
// analyzer reports in the file. When the fixes for its conversions through a
// dot import would leave that import unused, none of those fixes is offered.
func diagnose(pass *analysis.Pass, file *ast.File, names map[string]string, dots map[string]bool, s site, sites []site) analysis.Diagnostic {
	var (
		text, fixable = s.text, s.fixable
		path, dotted  = dotPath(pass, dots, s.call)
	)
	if dotted && !dotUsed(pass, file, path, sites, false) {
		text, fixable = "", false
	}
	message := s.typeName + " is a type, so the assignment sorts nothing"
	if text != "" {
		message += "; write " + text
	}
	d := analysis.Diagnostic{
		Pos:     s.assign.Pos(),
		End:     s.assign.End(),
		Message: message,
	}
	if fixable {
		edits := []analysis.TextEdit{{
			Pos:     s.assign.Pos(),
			End:     s.assign.End(),
			NewText: []byte(text),
		}}
		if _, imported := names["sort"]; !imported {
			edits = append(edits, imports.Add(pass, file, "sort"))
		}
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Write " + text + " (changes behavior)",
			TextEdits: edits,
		}}
	}
	return d
}

// same reports whether arg reads target, including a full slice of target.
func same(pass *analysis.Pass, target, arg ast.Expr) bool {
	if astexpr.Same(pass, target, arg) {
		return true
	}
	slice, ok := ast.Unparen(arg).(*ast.SliceExpr)
	return ok && slice.Low == nil && slice.High == nil &&
		!slice.Slice3 && astexpr.Same(pass, target, slice.X)
}

// sliceType reports whether t is an unnamed slice, or an alias of one.
func sliceType(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.Slice)
	return ok
}

// conversion returns the name of the function that sorts the elements and the
// name of the sort slice type when call converts a value to one of those
// types.
func conversion(pass *analysis.Pass, call *ast.CallExpr) (sortFunc, typeName string, ok bool) {
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return "", "", false
	}
	named, ok := types.Unalias(tv.Type).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "sort" {
		return "", "", false
	}
	sortFunc, ok = sorted[named.Obj().Name()]
	if !ok {
		return "", "", false
	}
	return sortFunc, "sort." + named.Obj().Name(), true
}

// replacement returns the text of a call to the matching function, qualified
// by the name the conversion uses for sort or another name the file imports
// sort under, or written with the bare function name under a dot import when
// the conversion does not name sort, and reports whether the assignment can be
// replaced with it. An empty string means the target is not addressable, a
// local declaration shadows the bare function name and no usable named import
// remains, the file imports sort under no usable name, or the package is sort
// or a package that sort imports, directly or transitively. A comment that
// overlaps the assignment keeps the text unless the dot-import check clears
// it; the fix is withheld in either case.
func replacement(pass *analysis.Pass, file *ast.File, names map[string]string, dots map[string]bool, sites []site, call *ast.CallExpr, assign *ast.AssignStmt, target ast.Expr, sortFunc string) (string, bool) {
	if tv, ok := pass.TypesInfo.Types[target]; ok && !tv.Addressable() {
		return "", false
	}
	if qualifier, ok := conversionQualifier(pass, call); ok {
		text := qualifier + "." + sortFunc + "(" + astexpr.Text(target) + ")"
		return text, !commented(file, assign.Pos(), assign.End())
	}
	if dots["sort"] {
		text := sortFunc + "(" + astexpr.Text(target) + ")"
		if unqualified(pass, file, sortFunc, assign.Pos()) {
			return text, !commented(file, assign.Pos(), assign.End())
		}
		name, named := importName(pass, file, assign.Pos())
		if named && dotUsed(pass, file, "sort", sites, true) {
			text = name + "." + sortFunc + "(" + astexpr.Text(target) + ")"
			return text, !commented(file, assign.Pos(), assign.End())
		}
		return "", false
	}
	name, ok := imports.Qualifier(pass, names, "sort", assign.Pos())
	if !ok {
		name, ok = importName(pass, file, assign.Pos())
	}
	if !ok {
		return "", false
	}
	if _, imported := names["sort"]; !imported && sortDep(pass.Pkg.Path()) {
		return "", false
	}
	text := name + "." + sortFunc + "(" + astexpr.Text(target) + ")"
	return text, !commented(file, assign.Pos(), assign.End())
}

// conversionQualifier returns the package name the conversion itself uses, and
// reports false when the conversion does not select a sort type by package.
func conversionQualifier(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return "", false
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "sort" {
		return "", false
	}
	return astexpr.Text(sel.X), true
}

// importName returns a usable name the file imports sort under.
func importName(pass *analysis.Pass, file *ast.File, pos token.Pos) (string, bool) {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "sort" || blankOrDot(spec.Name) {
			continue
		}
		name := implicitName(pass, spec)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "" {
			continue
		}
		object := imports.Lookup(pass, name, pos)
		pkg, ok := object.(*types.PkgName)
		if ok && pkg.Imported().Path() == "sort" {
			return name, true
		}
	}
	return "", false
}

// usableNames replaces a dot or blank name with a name another import of the
// same path binds. imports.Names keeps one name per path, so an import order
// can hide a name the rewrite can use.
func usableNames(pass *analysis.Pass, file *ast.File, names map[string]string) map[string]string {
	for _, spec := range file.Imports {
		if blankOrDot(spec.Name) {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := implicitName(pass, spec)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name != "" {
			names[path] = name
		}
	}
	return names
}

// blankOrDot reports whether id is the blank or dot import name.
func blankOrDot(id *ast.Ident) bool {
	return id != nil && (id.Name == "." || id.Name == "_")
}

// implicitName returns the name an unnamed import binds.
func implicitName(pass *analysis.Pass, spec *ast.ImportSpec) string {
	pkg, ok := pass.TypesInfo.Implicits[spec].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Name()
}

// dotPaths returns the paths the file imports with a dot. The names map
// holds one name per path, so it can miss a dot import beside another
// import of the same path.
func dotPaths(file *ast.File) map[string]bool {
	dots := make(map[string]bool)
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			dots[path] = true
		}
	}
	return dots
}

// dotPath returns the path of a package other than sort whose dot import
// supplies the type the conversion uses, and reports false when the type comes
// from sort or from no dot import.
func dotPath(pass *analysis.Pass, dots map[string]bool, call *ast.CallExpr) (string, bool) {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return "", false
	}
	name, ok := pass.TypesInfo.Uses[id].(*types.TypeName)
	if !ok || name.Pkg() == nil || name.Pkg().Path() == "sort" {
		return "", false
	}
	path := name.Pkg().Path()
	return path, dots[path]
}

// dotUsed reports whether the file uses an object from path outside the type
// names the fixes replace, a selector's member, and a keyed struct literal's
// field name. A dot import with no remaining use cannot stay, and goimports
// does not remove dot imports.
func dotUsed(pass *analysis.Pass, file *ast.File, path string, sites []site, all bool) bool {
	selected := make(map[*ast.Ident]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			selected[sel.Sel] = struct{}{}
		}
		return true
	})
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := selected[id]; ok {
			return true
		}
		for _, s := range sites {
			if !all && !s.fixable {
				continue
			}
			if s.call.Fun.Pos() <= id.Pos() && id.End() <= s.call.Fun.End() {
				return true
			}
		}
		object := pass.TypesInfo.Uses[id]
		if field, ok := object.(*types.Var); ok && field.IsField() {
			return true
		}
		if from(object, path) {
			found = true
			return false
		}
		return true
	})
	return found
}

// unqualified reports whether sortFunc reaches the sort package at pos in a
// file that imports it with a dot. A local declaration of the name reaches
// something else, so the rewrite would call that instead.
func unqualified(pass *analysis.Pass, file *ast.File, sortFunc string, pos token.Pos) bool {
	var expected types.Object
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() == "sort" {
			expected = pkg.Scope().Lookup(sortFunc)
			break
		}
	}
	if expected == nil {
		return false
	}
	scope := pass.TypesInfo.Scopes[file]
	if scope == nil {
		return false
	}
	if inner := scope.Innermost(pos); inner != nil {
		scope = inner
	}
	_, object := scope.LookupParent(sortFunc, pos)
	return object == expected
}

var (
	sortDepsOnce sync.Once
	sortDeps     map[string]bool
)

// sortDep reports whether path is the sort package or a package it imports,
// directly or transitively, where an import of sort would make a cycle.
func sortDep(path string) bool {
	sortDepsOnce.Do(func() {
		sortDeps = make(map[string]bool)
		var walk func(string)
		walk = func(path string) {
			if sortDeps[path] {
				return
			}
			sortDeps[path] = true
			pkg, err := build.Default.Import(path, "", 0)
			if err != nil {
				return
			}
			for _, imported := range pkg.Imports {
				walk(imported)
			}
		}
		walk("sort")
	})
	return sortDeps[path]
}

// from reports whether object is declared in the package at path.
func from(object types.Object, path string) bool {
	return object != nil && object.Pkg() != nil && object.Pkg().Path() == path
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
