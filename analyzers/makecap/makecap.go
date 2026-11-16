// Package makecap provides an analysis.Analyzer that reports a capacity
// argument equal to make's default.
//
// make's documentation says "The capacity of the slice is equal to its
// length", so a slice's third argument is redundant when it repeats the
// second. A channel's buffer capacity is zero when its size is omitted, so a
// constant zero there is redundant too.
//
// A slice's length and capacity are compared as values. They match when they
// are the same identifier, field, literal, index, or dereference, when their
// constant values are equal, or when they are the same expression whose value
// cannot change between the two evaluations. A call that can return a
// different value is left alone, and so is a length that reads a channel,
// which another goroutine can change.
//
// The fix deletes the capacity argument. It is withheld when a comment is in
// the text the fix deletes, or when the fixes together would leave a local
// variable or an import unused.
//
// A map's size argument and a make call whose type argument is a type
// parameter are left alone. The README catalogs every check, exemption, and
// fix.
//
// The rule is adapted from staticcheck's S1019, which simplifies a make call
// by omitting redundant arguments.
package makecap

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "makecap",
	Doc:  "reports a make capacity argument equal to the default",
	Run:  run,
}

// candidate is one capacity argument the analyzer reports.
type candidate struct {
	file     *ast.File
	facts    *facts
	start    token.Pos
	capacity ast.Expr
	repeats  bool
	message  string
}

// facts are the file-wide maps every candidate in a file shares.
type facts struct {
	dots      map[*types.Package]struct{}
	selectors map[*ast.Ident]struct{}
	writes    map[*ast.Ident]bool
}

func run(pass *analysis.Pass) (any, error) {
	var (
		found []candidate
		known = make(map[*ast.File]*facts)
	)
	for _, file := range pass.Files {
		known[file] = newFacts(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isMake(pass, call) {
				return true
			}
			typ := pass.TypesInfo.TypeOf(call)
			if typ == nil {
				return true
			}
			switch typ.Underlying().(type) {
			case *types.Slice:
				if c, ok := sliceCandidate(pass, file, call); ok {
					c.facts = known[file]
					found = append(found, c)
				}
			case *types.Chan:
				if c, ok := chanCandidate(pass, file, call); ok {
					c.facts = known[file]
					found = append(found, c)
				}
			}
			return true
		})
	}
	for _, c := range found {
		report(pass, c, found)
	}
	return nil, nil
}

// sliceCandidate returns a slice whose length and capacity are the same value.
func sliceCandidate(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (candidate, bool) {
	if len(call.Args) != 3 {
		return candidate{}, false
	}
	same, repeats := sameValue(pass, call.Args[1], call.Args[2])
	if !same {
		return candidate{}, false
	}
	return candidate{
		file:     file,
		start:    call.Args[1].End(),
		capacity: call.Args[2],
		repeats:  repeats,
		message:  "capacity can be omitted; it equals the length",
	}, true
}

// chanCandidate returns a channel whose buffer capacity is the constant zero.
func chanCandidate(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (candidate, bool) {
	if len(call.Args) != 2 || !zero(pass, call.Args[1]) {
		return candidate{}, false
	}
	return candidate{
		file:     file,
		start:    call.Args[0].End(),
		capacity: call.Args[1],
		message:  "capacity can be omitted; zero is unbuffered",
	}, true
}

// report suggests deleting the text from start to the capacity argument.
func report(pass *analysis.Pass, c candidate, found []candidate) {
	diag := analysis.Diagnostic{
		Pos:     c.capacity.Pos(),
		End:     c.capacity.End(),
		Message: c.message,
	}
	if fixable(pass, c, found) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Omit the capacity",
			TextEdits: []analysis.TextEdit{{
				Pos: c.start,
				End: c.capacity.End(),
			}},
		}}
	}
	pass.Report(diag)
}

// fixable reports whether deleting capacity leaves its names in use. Repeating
// the length is enough, since the length remains.
func fixable(pass *analysis.Pass, c candidate, found []candidate) bool {
	if commented(c.file, c.start, c.capacity.End()) {
		return false
	}
	return c.repeats || !orphaned(pass, c, found)
}

// orphaned reports whether capacity uses a local variable or an import that
// nothing else in the file uses. Deleting the capacity argument would leave
// that name unused, which does not compile. A use in another candidate's
// capacity does not count, since the run may delete that capacity too.
func orphaned(pass *analysis.Pass, c candidate, found []candidate) bool {
	locals, named, dots := orphanTargets(pass, c.capacity, c.facts)
	if len(locals) == 0 && len(named) == 0 && len(dots) == 0 {
		return false
	}
	selectors := c.facts.selectors
	writes := c.facts.writes
	for id, obj := range pass.TypesInfo.Uses {
		if id.Pos() < c.file.FileStart || id.End() > c.file.FileEnd {
			continue
		}
		if deleted(found, id) {
			continue
		}
		if _, ok := locals[obj]; ok {
			if !writes[id] {
				delete(locals, obj)
			}
			continue
		}
		if _, ok := named[obj]; ok {
			delete(named, obj)
			continue
		}
		if _, ok := selectors[id]; ok {
			continue
		}
		if v, ok := obj.(*types.Var); ok && v.Kind() == types.FieldVar {
			continue
		}
		if obj.Pkg() != nil {
			delete(dots, obj.Pkg())
		}
	}
	return len(locals)+len(named)+len(dots) > 0
}

// deleted reports whether id lies in one of the capacity arguments in found.
func deleted(found []candidate, id *ast.Ident) bool {
	for _, c := range found {
		if id.Pos() >= c.capacity.Pos() && id.End() <= c.capacity.End() {
			return true
		}
	}
	return false
}

// orphanTargets returns the local variables, named imports, and dot-imported
// packages that capacity uses.
func orphanTargets(pass *analysis.Pass, capacity ast.Expr, f *facts) (locals, named map[types.Object]struct{}, dots map[*types.Package]struct{}) {
	locals = make(map[types.Object]struct{})
	named = make(map[types.Object]struct{})
	dots = make(map[*types.Package]struct{})
	var (
		dot       = f.dots
		selectors = f.selectors
	)
	ast.Inspect(capacity, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		addDot := func(obj types.Object) {
			if obj == nil || !packageName(obj) {
				return
			}
			if _, ok := selectors[id]; ok {
				return
			}
			if _, ok := dot[obj.Pkg()]; ok {
				dots[obj.Pkg()] = struct{}{}
			}
		}
		switch obj := pass.TypesInfo.ObjectOf(id).(type) {
		case *types.Var:
			if obj.Kind() == types.LocalVar {
				locals[obj] = struct{}{}
				return true
			}
			addDot(obj)
		case *types.PkgName:
			named[obj] = struct{}{}
		default:
			addDot(obj)
		}
		return true
	})
	return locals, named, dots
}

// packageName reports whether obj is a package-level name: a variable,
// constant, type, or function. A struct field and a method are not.
func packageName(obj types.Object) bool {
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	switch obj := obj.(type) {
	case *types.Var:
		return obj.Kind() == types.PackageVar
	case *types.Const, *types.TypeName:
		return true
	case *types.Func:
		return obj.Signature().Recv() == nil
	}
	return false
}

// newFacts builds the maps a file's candidates share.
func newFacts(pass *analysis.Pass, file *ast.File) *facts {
	return &facts{
		dots:      dotImports(pass, file),
		selectors: selectorNames(file),
		writes:    written(file),
	}
}

// dotImports returns the packages file imports with a dot name.
func dotImports(pass *analysis.Pass, file *ast.File) map[*types.Package]struct{} {
	out := make(map[*types.Package]struct{})
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		if pkg := pass.TypesInfo.PkgNameOf(spec); pkg != nil {
			out[pkg.Imported()] = struct{}{}
		}
	}
	return out
}

// selectorNames returns the identifiers on the right of a selector
// expression. A method or field selector is not a package-level name, so a dot
// import is not in use there.
func selectorNames(file *ast.File) map[*ast.Ident]struct{} {
	out := make(map[*ast.Ident]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			out[sel.Sel] = struct{}{}
		}
		return true
	})
	return out
}

// written collects the identifiers an assignment or a short declaration
// writes. x++ and x += y also read x, so they are not writes for this purpose.
func written(file *ast.File) map[*ast.Ident]bool {
	out := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
				return true
			}
			for _, lhs := range n.Lhs {
				if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					out[id] = true
				}
			}
		case *ast.RangeStmt:
			if n.Tok != token.ASSIGN {
				return true
			}
			for _, e := range []ast.Expr{n.Key, n.Value} {
				if e == nil {
					continue
				}
				if id, ok := ast.Unparen(e).(*ast.Ident); ok {
					out[id] = true
				}
			}
		}
		return true
	})
	return out
}

// isMake reports whether call calls the builtin make.
func isMake(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("make")
}

// sameValue reports whether a and b are the same value, and whether b repeats
// a, so deleting b leaves a behind.
func sameValue(pass *analysis.Pass, a, b ast.Expr) (same, repeats bool) {
	if astexpr.Same(pass, a, b) {
		return true, true
	}
	if astexpr.SamePure(pass, a, b) {
		stable := astexpr.StableLength(pass, a)
		return stable, stable
	}
	ak, bk := integer(pass, a), integer(pass, b)
	if ak != nil && bk != nil {
		return constant.Compare(ak, token.EQL, bk), false
	}
	return false, false
}

// zero reports whether e is the constant zero.
func zero(pass *analysis.Pass, e ast.Expr) bool {
	v := integer(pass, e)
	return v != nil && constant.Sign(v) == 0
}

// integer returns e's value as an integer constant, or nil when e is not one.
func integer(pass *analysis.Pass, e ast.Expr) constant.Value {
	v := pass.TypesInfo.Types[e].Value
	if v == nil {
		return nil
	}
	v = constant.ToInt(v)
	if v.Kind() != constant.Int {
		return nil
	}
	return v
}

// commented reports whether a comment lies in the text from lo to hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
