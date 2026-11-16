// Package sortstrings provides an analysis.Analyzer that reports calls that
// sort a slice of an ordered element type with the sort package.
//
// The slices package doc says Sort "sorts a slice of any ordered type in
// ascending order" (https://pkg.go.dev/slices#Sort). That is the order
// sort.Strings, sort.Ints, and sort.Float64s give their element types, and
// the order a sort.Slice call gives when its less function compares the two
// elements at the indexes it was given.
//
// A sort.Slice over an element type that can hold a NaN is left alone, since
// slices.Sort places NaNs first and the supplied comparison does not.
//
// The fix writes slices.Sort(x), adding the import where the file lacks one.
// A file below Go 1.21 is left alone. The fix is withheld for a comment inside
// the call, a dot import of sort that the replacement would leave unused, a
// name used for slices at the call that reaches another object, a dot or blank
// import of slices, and a package that slices imports, where the new import
// would make a cycle.
//
// The README catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's S1032, which reports
// sort.Sort(sort.StringSlice(x)), sort.Sort(sort.IntSlice(x)), and
// sort.Sort(sort.Float64Slice(x)) in favor of the element-typed functions.
// This rule reports those element-typed functions, and the sort.Slice form
// with a slice argument and an element type that cannot hold a NaN, in favor
// of slices.Sort. The suite runs this rule in place of slicessort, whose fix
// can delete a comment and move a NaN, and which also reported sort.Slice over
// a NaN-capable element type and over arguments this rule leaves alone.
package sortstrings

import (
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"go/version"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "sortstrings",
	Doc:  "reports sort.Strings, sort.Ints, sort.Float64s, and a sort.Slice over a slice of an ordered type that cannot hold a NaN",
	Run:  run,
}

// checker holds one file of the package.
type checker struct {
	pass      *analysis.Pass
	file      *ast.File
	names     map[string]string
	dotUnused bool
}

// site is one call the rule reports.
type site struct {
	call *ast.CallExpr
	arg  ast.Expr
	name string
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !modern(pass, file) {
			continue
		}
		var sites []site
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if arg, name, ok := covered(pass, call); ok {
				sites = append(sites, site{call, arg, name})
			}
			return true
		})
		c := checker{pass: pass, file: file, names: imports.Names(pass, file)}
		c.dotUnused = c.names["sort"] == "." && !c.otherSortUse(sites)
		for _, s := range sites {
			pass.Report(c.diagnose(s.call, s.arg, s.name))
		}
	}
	return nil, nil
}

// modern reports whether the file is checked against Go 1.21 or later, where
// slices.Sort exists. An empty version means the type checker enforced none.
func modern(pass *analysis.Pass, file *ast.File) bool {
	v := pass.TypesInfo.FileVersions[file]
	return v == "" || version.Compare(v, "go1.21") >= 0
}

// covered returns the slice and the name of the call when the call is one the
// rule reports, and reports false for any other call.
func covered(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, string, bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "sort" {
		return nil, "", false
	}
	switch fn.Name() {
	case "Strings", "Ints", "Float64s":
		if len(call.Args) != 1 || nilArg(pass.TypesInfo.TypeOf(call.Args[0])) {
			return nil, "", false
		}
		return call.Args[0], "sort." + fn.Name(), true
	case "Slice":
		if len(call.Args) != 2 {
			return nil, "", false
		}
		elem, ok := element(pass.TypesInfo.TypeOf(call.Args[0]))
		if !ok || astexpr.MayBeNaN(elem) {
			return nil, "", false
		}
		if !direct(pass, call.Args[0], call.Args[1]) {
			return nil, "", false
		}
		return call.Args[0], "sort.Slice", true
	}
	return nil, "", false
}

// direct reports whether less is a func literal whose only statement returns
// the comparison of the two elements of slice at the indexes it was given.
func direct(pass *analysis.Pass, slice, less ast.Expr) bool {
	lit, ok := ast.Unparen(less).(*ast.FuncLit)
	if !ok || lit.Body == nil || len(lit.Body.List) != 1 {
		return false
	}
	ret, ok := lit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	cmp, ok := ast.Unparen(ret.Results[0]).(*ast.BinaryExpr)
	if !ok || cmp.Op != token.LSS {
		return false
	}
	sig, ok := pass.TypesInfo.TypeOf(lit.Type).(*types.Signature)
	if !ok || sig.Params().Len() != 2 {
		return false
	}
	return at(pass, cmp.X, slice, sig.Params().At(0)) &&
		at(pass, cmp.Y, slice, sig.Params().At(1))
}

// element returns the element type of t where every type in its type set is a
// slice with that element type.
func element(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}
	switch t := t.(type) {
	case *types.TypeParam:
		return element(t.Constraint())
	case *types.Union:
		return unionElement(t)
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		var elem types.Type
		for embedded := range iface.EmbeddedTypes() {
			if unrestricted(embedded) {
				continue
			}
			e, ok := element(embedded)
			if !ok || elem != nil && !types.Identical(elem, e) {
				return nil, false
			}
			elem = e
		}
		return elem, elem != nil
	}
	slice, ok := t.Underlying().(*types.Slice)
	if !ok {
		return nil, false
	}
	return slice.Elem(), true
}

// unionElement returns the element type of a union whose terms are slices with
// one element type.
func unionElement(union *types.Union) (types.Type, bool) {
	var elem types.Type
	for term := range union.Terms() {
		e, ok := element(term.Type())
		if !ok || elem != nil && !types.Identical(elem, e) {
			return nil, false
		}
		elem = e
	}
	return elem, elem != nil
}

// unrestricted reports whether t names no type of its own, as comparable and
// any do.
func unrestricted(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	return ok && iface.NumEmbeddeds() == 0
}

// at reports whether e is an element of slice at the index named by param.
func at(pass *analysis.Pass, e, slice ast.Expr, param *types.Var) bool {
	index, ok := ast.Unparen(e).(*ast.IndexExpr)
	if !ok || !astexpr.Same(pass, index.X, slice) {
		return false
	}
	id, ok := ast.Unparen(index.Index).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == param
}

// diagnose returns the diagnostic for one call, with the rewrite as a fix
// where the rewrite can be written.
func (c checker) diagnose(call *ast.CallExpr, arg ast.Expr, name string) analysis.Diagnostic {
	d := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: name + " can be replaced by slices.Sort",
	}
	if fix, ok := c.fix(call, arg); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{fix}
	}
	return d
}

// fix returns the edit that replaces the call with slices.Sort of its argument
// and adds the import unless the file has it.
func (c checker) fix(call *ast.CallExpr, arg ast.Expr) (analysis.SuggestedFix, bool) {
	if commented(c.file, call.Pos(), call.End()) {
		return analysis.SuggestedFix{}, false
	}
	if slicesDep(c.pass.Pkg.Path()) {
		return analysis.SuggestedFix{}, false
	}
	if c.dotUnused {
		return analysis.SuggestedFix{}, false
	}
	name, ok := imports.Qualifier(c.pass, c.names, "slices", call.Pos())
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	text := name + ".Sort(" + astexpr.Text(arg) + ")"
	edits := []analysis.TextEdit{{
		Pos:     call.Pos(),
		End:     call.End(),
		NewText: []byte(text),
	}}
	if _, imported := c.names["slices"]; !imported {
		edits = append(edits, imports.Add(c.pass, c.file, "slices"))
	}
	return analysis.SuggestedFix{
		Message:   "Replace with " + text,
		TextEdits: edits,
	}, true
}

// otherSortUse reports whether the file names an object from sort outside the
// parts of sites that a fix removes, so a dot import of sort stays used.
func (c checker) otherSortUse(sites []site) bool {
	var found bool
	ast.Inspect(c.file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		for _, s := range sites {
			var (
				inCall = s.call.Pos() <= id.Pos() && id.End() <= s.call.End()
				inArg  = s.arg.Pos() <= id.Pos() && id.End() <= s.arg.End()
			)
			if inCall && !inArg {
				return true
			}
		}
		obj := c.pass.TypesInfo.Uses[id]
		if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "sort" {
			found = true
			return false
		}
		return true
	})
	return found
}

var (
	sliceDepsOnce sync.Once
	sliceDeps     map[string]bool
)

// slicesDep reports whether path is the slices package or one of its imports,
// where an import of slices would make a cycle.
func slicesDep(path string) bool {
	sliceDepsOnce.Do(func() {
		sliceDeps = make(map[string]bool)
		var walk func(string)
		walk = func(path string) {
			if sliceDeps[path] {
				return
			}
			sliceDeps[path] = true
			pkg, err := build.Default.Import(path, "", 0)
			if err != nil {
				return
			}
			for _, imported := range pkg.Imports {
				walk(imported)
			}
		}
		walk("slices")
	})
	return sliceDeps[path]
}

// nilArg reports whether t is the type of an untyped nil, which gives
// slices.Sort no element type to infer.
func nilArg(t types.Type) bool {
	return t != nil && types.Identical(t, types.Typ[types.UntypedNil])
}

// commented reports whether a comment overlaps the text between lo and hi,
// where the edit would delete it with the call.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
