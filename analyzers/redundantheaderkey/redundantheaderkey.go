// Package redundantheaderkey provides an analysis.Analyzer that reports a call
// to http.CanonicalHeaderKey on the key argument of an http.Header method.
//
// The net/http Header doc says the keys "should be in canonical form, as
// returned by CanonicalHeaderKey". The docs for Add, Set, Get, Del, and Values
// on that page say the method canonicalizes the key
// (https://pkg.go.dev/net/http#Header.Add), so the call is unnecessary.
//
// The call is reported when it is the key argument of a direct call to one of
// those methods, including a method expression and a promoted method. A
// CanonicalHeaderKey call on a value argument is left alone, as are a
// CanonicalHeaderKey used for a direct map access, one held in a function
// value, and a method value called through a variable. The fix replaces the
// call with its argument. It is withheld when a comment lies in the text it
// would replace, when the fixes together would leave a dot import of net/http
// unused, or when the key spans more than one line. A nested call may also be
// reported without a fix so two suggested fixes do not overlap.
//
// The rule is adapted from staticcheck's S1035, which reports the same call
// for Add, Del, Get, and Set.
package redundantheaderkey

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "redundantheaderkey",
	Doc:  "reports a redundant http.CanonicalHeaderKey on an http.Header key argument",
	Run:  run,
}

// keyMethods are the http.Header methods that canonicalize their key
// argument.
var keyMethods = map[string]struct{}{
	"Add":    {},
	"Del":    {},
	"Get":    {},
	"Set":    {},
	"Values": {},
}

type site struct {
	key     ast.Expr
	inner   *ast.CallExpr
	method  string
	fixable bool
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var sites []site
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if found, ok := match(pass, file, call); ok {
				sites = append(sites, found)
			}
			return true
		})
		disjoint(sites)
		dotUnused := dotImported(pass, file) &&
			!otherNetHTTPUse(pass, file, sites)
		for _, found := range sites {
			report(pass, found, dotUnused)
		}
	}
	return nil, nil
}

// match returns a site when call is an http.Header method whose key wraps a
// CanonicalHeaderKey call.
func match(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (site, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return site{}, false
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection == nil {
		return site{}, false
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok || !headerMethod(fn) {
		return site{}, false
	}
	var keyAt int
	switch selection.Kind() {
	case types.MethodExpr:
		keyAt = 1
	case types.MethodVal:
	default:
		return site{}, false
	}
	if keyAt >= len(call.Args) {
		return site{}, false
	}
	key := call.Args[keyAt]
	inner, ok := canonical(pass, key)
	if !ok {
		return site{}, false
	}
	return site{
		key:    key,
		inner:  inner,
		method: fn.Name(),
		fixable: !commented(file, key.Pos(), key.End()) &&
			oneLine(pass, key.Pos(), key.End()),
	}, true
}

// report adds a diagnostic for a site. When the site is fixable and the fixes
// do not leave a dot import of net/http unused, the diagnostic carries a fix
// that removes the CanonicalHeaderKey call.
func report(pass *analysis.Pass, found site, dotUnused bool) {
	diagnostic := analysis.Diagnostic{
		Pos: found.inner.Pos(),
		End: found.inner.End(),
		Message: "http.Header." + found.method + " canonicalizes the key; " +
			"http.CanonicalHeaderKey is redundant",
	}
	if found.fixable && !dotUnused {
		arg := ast.Unparen(found.inner.Args[0])
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Remove the CanonicalHeaderKey call",
			TextEdits: []analysis.TextEdit{{
				Pos:     found.key.Pos(),
				End:     found.key.End(),
				NewText: []byte(astexpr.Text(arg)),
			}},
		}}
	}
	pass.Report(diagnostic)
}

// oneLine reports whether the range from pos to end stays on one written line.
func oneLine(pass *analysis.Pass, pos, end token.Pos) bool {
	var (
		start = pass.Fset.PositionFor(pos, false)
		stop  = pass.Fset.PositionFor(end, false)
	)
	return start.Line == stop.Line
}

// disjoint withholds the fix for a site nested inside the key of another
// fixable site, so two suggested fixes never overlap.
func disjoint(sites []site) {
	eligible := make([]bool, len(sites))
	for i := range sites {
		eligible[i] = sites[i].fixable
	}
	for i := range sites {
		sites[i].fixable = eligible[i]
		for j := range sites {
			if i == j || !eligible[j] {
				continue
			}
			if inside(sites[i].key, sites[j].key) {
				sites[i].fixable = false
				break
			}
		}
	}
}

// inside reports whether inner lies within outer without covering the same
// text.
func inside(inner, outer ast.Expr) bool {
	within := outer.Pos() <= inner.Pos() && inner.End() <= outer.End()
	return within && (inner.Pos() != outer.Pos() || inner.End() != outer.End())
}

// dotImported reports whether file imports net/http with a dot.
func dotImported(pass *analysis.Pass, file *ast.File) bool {
	for _, spec := range file.Imports {
		obj := pass.TypesInfo.Defs[spec.Name]
		if obj == nil {
			obj = pass.TypesInfo.Implicits[spec]
		}
		pkg, ok := obj.(*types.PkgName)
		if ok && pkg.Imported().Path() == "net/http" && pkg.Name() == "." {
			return true
		}
	}
	return false
}

// otherNetHTTPUse reports whether a dot-imported name from net/http appears
// outside the calls the fixes remove, so the dot import stays used.
func otherNetHTTPUse(pass *analysis.Pass, file *ast.File, sites []site) bool {
	qualified := make(map[*ast.Ident]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			qualified[sel.Sel] = struct{}{}
		}
		return true
	})
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := qualified[id]; ok {
			return true
		}
		obj := pass.TypesInfo.Uses[id]
		if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "net/http" {
			return true
		}
		if obj.Parent() != obj.Pkg().Scope() {
			return true
		}
		if !removed(id, sites) {
			found = true
			return false
		}
		return true
	})
	return found
}

// removed reports whether a fix deletes the identifier, which lies in a key
// but outside the argument the fix keeps.
func removed(id *ast.Ident, sites []site) bool {
	for _, found := range sites {
		if !found.fixable {
			continue
		}
		if id.Pos() < found.key.Pos() || id.End() > found.key.End() {
			continue
		}
		arg := found.inner.Args[0]
		return id.Pos() < arg.Pos() || id.End() > arg.End()
	}
	return false
}

// headerMethod reports whether fn is an http.Header method that canonicalizes
// its key.
func headerMethod(fn *types.Func) bool {
	if fn.Pkg() == nil || fn.Pkg().Path() != "net/http" {
		return false
	}
	if _, ok := keyMethods[fn.Name()]; !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	t := types.Unalias(sig.Recv().Type())
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	obj := named.Obj()
	return obj.Pkg().Path() == "net/http" && obj.Name() == "Header"
}

// canonical returns the call to http.CanonicalHeaderKey in expr.
func canonical(pass *analysis.Pass, expr ast.Expr) (*ast.CallExpr, bool) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "net/http" {
		return nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	return call, ok && sig.Recv() == nil && fn.Name() == "CanonicalHeaderKey"
}

// commented reports whether a comment sits in the span a fix would replace.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
