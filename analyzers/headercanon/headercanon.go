// Package headercanon provides an analysis.Analyzer that reports a key in an
// http.Header map that is not in canonical form.
//
// The net/http Header doc says the keys "should be in canonical form, as
// returned by CanonicalHeaderKey". Set says "To use non-canonical keys, assign
// to the map directly", and Get and Values say the same about access. A key
// in a variable is left alone, so direct map access remains available; a
// constant key is reported because it is not canonical.
//
// The fix is withheld for a direct key to an http.Header, by index or delete,
// when the key appears noncanonically in a header-shaped composite literal in
// the package, the package has another direct key the rule does not report and
// cannot prove canonical, a nonconstant key the rule cannot prove canonical
// appears in a header-shaped composite literal, or the canonicalizer for a
// nonliteral key cannot be named at the key's position. The fix replaces a
// string literal with its canonical form, wraps any other constant in
// http.CanonicalHeaderKey, and adds the import when the file has none. A
// parenthesized literal that carries a comment is edited in place: only the
// literal changes. A call to http.CanonicalHeaderKey or
// textproto.CanonicalMIMEHeaderKey is treated as canonical. The README lists
// these cases.
//
// A composite literal key is not reported: the key may be deliberate. An empty
// key is already canonical, and a key with a space, or any invalid header
// field byte, is left alone because CanonicalHeaderKey returns it "without
// modifications".
//
// The rule is adapted from staticcheck's SA1008.
package headercanon

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"net/http"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "headercanon",
	Doc:  "reports a non-canonical key in an http.Header map",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	literal, dynamic := guards(pass)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.IndexExpr:
				if pass.TypesInfo.Types[n].IsType() {
					return true
				}
				if isHeader(pass.TypesInfo.TypeOf(n.X)) {
					check(pass, file, n.Index, literal, dynamic)
				}
			case *ast.CallExpr:
				if key, ok := deleted(pass, n); ok {
					check(pass, file, key, literal, dynamic)
				}
			}
			return true
		})
	}
	return nil, nil
}

// deleteKey returns the key argument of a builtin delete call.
func deleteKey(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || len(call.Args) != 2 {
		return nil, false
	}
	if pass.TypesInfo.Uses[id] != types.Universe.Lookup("delete") {
		return nil, false
	}
	return call.Args[1], true
}

// deleted returns the key of a delete call on an http.Header.
func deleted(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	key, ok := deleteKey(pass, call)
	if !ok || !isHeader(pass.TypesInfo.TypeOf(call.Args[0])) {
		return nil, false
	}
	return key, true
}

// guards returns the noncanonical keys a header-shaped composite literal in
// the package holds, and whether the package has a key the rule does not
// report and cannot prove canonical. A matching literal key withholds that
// key's fix; dynamic withholds every fix in the package.
func guards(pass *analysis.Pass) (literal map[string]struct{}, dynamic bool) {
	literal = make(map[string]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CompositeLit:
				if !headerMap(pass.TypesInfo.TypeOf(n)) {
					return true
				}
				for _, elt := range n.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok || canonicalKey(pass, kv.Key) {
						continue
					}
					if text, ok := constantText(pass, kv.Key); ok {
						literal[text] = struct{}{}
					} else {
						dynamic = true
					}
				}
			case *ast.IndexExpr:
				if pass.TypesInfo.Types[n].IsType() {
					return true
				}
				dynamic = dynamic || uncontrolled(pass, n.X, n.Index)
			case *ast.CallExpr:
				if key, ok := deleteKey(pass, n); ok {
					dynamic = dynamic || uncontrolled(pass, n.Args[0], key)
				}
			}
			return !dynamic
		})
		if dynamic {
			return literal, true
		}
	}
	return literal, false
}

// uncontrolled reports whether the key at an access to h cannot be proved
// canonical. A canonical key, a canonicalizer call, and a constant key on
// http.Header are controlled. Any other nonconstant key on http.Header, and a
// key on a header-shaped map the rule does not report and cannot prove
// canonical, are not.
func uncontrolled(pass *analysis.Pass, h, key ast.Expr) bool {
	if canonicalKey(pass, key) {
		return false
	}
	t := pass.TypesInfo.TypeOf(h)
	if isHeader(t) {
		_, ok := constantText(pass, key)
		return !ok
	}
	return headerMap(t)
}

// canonicalKey reports whether key is a canonical string or the result of a
// canonicalizer call.
func canonicalKey(pass *analysis.Pass, key ast.Expr) bool {
	if text, ok := constantText(pass, key); ok {
		return http.CanonicalHeaderKey(text) == text
	}
	call, ok := ast.Unparen(key).(*ast.CallExpr)
	if !ok {
		return false
	}
	var fn *types.Func
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		fn, _ = pass.TypesInfo.Uses[fun].(*types.Func)
	case *ast.SelectorExpr:
		fn, _ = pass.TypesInfo.Uses[fun.Sel].(*types.Func)
	}
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "net/http":
		return fn.Name() == "CanonicalHeaderKey"
	case "net/textproto":
		return fn.Name() == "CanonicalMIMEHeaderKey"
	}
	return false
}

// headerMap reports whether every type term of t is a map from string to
// []string, which includes http.Header, textproto.MIMEHeader, a plain
// map[string][]string, and defined equivalents.
func headerMap(t types.Type) bool {
	terms, universal := headerTerms(t)
	if universal || len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if !headerMapType(term.typ) {
			return false
		}
	}
	return true
}

// headerMapType reports whether t is a map from string to []string.
func headerMapType(t types.Type) bool {
	m, ok := t.Underlying().(*types.Map)
	if !ok || !types.Identical(m.Key(), types.Typ[types.String]) {
		return false
	}
	return types.Identical(m.Elem(), types.NewSlice(types.Typ[types.String]))
}

// headerTerm is one element of a type set. An exact term names one type, and
// the type of an approximate term is its underlying type.
type headerTerm struct {
	typ   types.Type
	exact bool
}

// headerTerms returns the type terms of t, and whether t supplies no type
// terms at all. Embedded elements are intersected.
func headerTerms(t types.Type) ([]headerTerm, bool) {
	t = types.Unalias(t)
	if union, ok := t.(*types.Union); ok {
		var out []headerTerm
		for term := range union.Terms() {
			if term.Tilde() {
				out = append(out, headerTerm{typ: term.Type().Underlying()})
				continue
			}
			terms, universal := headerTerms(term.Type())
			if universal {
				return nil, true
			}
			out = append(out, terms...)
		}
		return headerUnique(out), false
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return []headerTerm{{typ: t, exact: true}}, false
	}
	var terms []headerTerm
	universal := true
	for embedded := range iface.EmbeddedTypes() {
		narrowed, none := headerTerms(embedded)
		if none {
			continue
		}
		if universal {
			terms, universal = narrowed, false
		} else {
			terms = headerIntersect(terms, narrowed)
		}
	}
	return terms, universal
}

// headerIntersect returns the terms shared by a and b.
func headerIntersect(a, b []headerTerm) []headerTerm {
	var out []headerTerm
	for _, x := range a {
		for _, y := range b {
			if t, ok := headerMeet(x, y); ok {
				out = append(out, t)
			}
		}
	}
	return headerUnique(out)
}

// headerMeet returns the term common to a and b.
func headerMeet(a, b headerTerm) (headerTerm, bool) {
	switch {
	case a.exact && b.exact:
		return a, types.Identical(a.typ, b.typ)
	case a.exact && !b.exact:
		return a, types.Identical(a.typ.Underlying(), b.typ)
	case !a.exact && b.exact:
		return b, types.Identical(a.typ, b.typ.Underlying())
	}
	return a, types.Identical(a.typ, b.typ)
}

// headerUnique drops duplicate terms.
func headerUnique(terms []headerTerm) (out []headerTerm) {
	for _, t := range terms {
		var dup bool
		for _, u := range out {
			if t.exact == u.exact && types.Identical(t.typ, u.typ) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, t)
		}
	}
	return out
}

// check reports a constant key whose value is not canonical.
func check(pass *analysis.Pass, file *ast.File, key ast.Expr, literal map[string]struct{}, dynamic bool) {
	text, ok := constantText(pass, key)
	if !ok {
		return
	}
	canonical := http.CanonicalHeaderKey(text)
	if text == canonical {
		return
	}
	diagnostic := analysis.Diagnostic{
		Pos:     key.Pos(),
		End:     key.End(),
		Message: "header key " + strconv.Quote(text) + " is not canonical",
	}
	if _, held := literal[text]; !held && !dynamic {
		if edits := edits(pass, file, key, canonical); len(edits) > 0 {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message:   "Canonicalize the key",
				TextEdits: edits,
			}}
		}
	}
	pass.Report(diagnostic)
}

// constantText returns the string value of key, if it is a string constant.
func constantText(pass *analysis.Pass, key ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[key]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// edits returns the edits that rewrite key to its canonical form. A literal
// becomes the canonical literal. Any other constant keeps its expression and
// is passed to http.CanonicalHeaderKey, which may need an import.
func edits(pass *analysis.Pass, file *ast.File, key ast.Expr, canonical string) []analysis.TextEdit {
	lit, ok := ast.Unparen(key).(*ast.BasicLit)
	if ok && lit.Kind == token.STRING {
		pos, end := key.Pos(), key.End()
		if lit != key && commented(file, pos, end) {
			pos, end = lit.Pos(), lit.End()
		}
		return []analysis.TextEdit{{
			Pos:     pos,
			End:     end,
			NewText: []byte(strconv.Quote(canonical)),
		}}
	}
	call, add, ok := canonicalCall(pass, file, key.Pos())
	if !ok {
		return nil
	}
	src, err := pass.ReadFile(pass.Fset.File(file.Pos()).Name())
	if err != nil {
		return nil
	}
	keyText, ok := text(pass, src, key)
	if !ok {
		return nil
	}
	out := []analysis.TextEdit{{
		Pos:     key.Pos(),
		End:     key.End(),
		NewText: []byte(call + "(" + keyText + ")"),
	}}
	if add {
		out = append(out, imports.Add(pass, file, "net/http"))
	}
	return out
}

// canonicalCall returns the call that canonicalizes a key at pos, whether
// writing it needs an import for net/http, and whether the call can be named
// there.
func canonicalCall(pass *analysis.Pass, file *ast.File, pos token.Pos) (call string, add, ok bool) {
	names := imports.Names(pass, file)
	if pass.Pkg.Path() == "net/http" || names["net/http"] == "." {
		if referred(pass, pos) {
			return "CanonicalHeaderKey", false, true
		}
		return "", false, false
	}
	qualifier, ok := imports.Qualifier(pass, names, "net/http", pos)
	if !ok {
		return "", false, false
	}
	_, imported := names["net/http"]
	return qualifier + ".CanonicalHeaderKey", !imported, true
}

// referred reports whether the unqualified CanonicalHeaderKey reaches net/http
// at pos.
func referred(pass *analysis.Pass, pos token.Pos) bool {
	fn, ok := imports.Lookup(pass, "CanonicalHeaderKey", pos).(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "net/http"
}

// isHeader reports whether t's type set is exactly net/http.Header, through
// any alias, type parameter, or constraint.
func isHeader(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	terms, universal := headerTerms(t)
	if universal {
		return false
	}
	iface, hasInterface := t.Underlying().(*types.Interface)
	var found *types.Named
	for _, term := range terms {
		if !term.exact {
			return false
		}
		if hasInterface && !types.Satisfies(term.typ, iface) {
			continue
		}
		named, ok := term.typ.(*types.Named)
		if !ok || !headerNamed(named) || found != nil {
			return false
		}
		found = named
	}
	return found != nil
}

// headerNamed reports whether named is net/http.Header.
func headerNamed(named *types.Named) bool {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == "net/http" && obj.Name() == "Header"
}

// commented reports whether a comment lies within lo to hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && lo < group.End() {
			return true
		}
	}
	return false
}

// text returns the source text of e, keeping any comments between its tokens.
// It reports false when the file's bytes do not cover e.
func text(pass *analysis.Pass, src []byte, e ast.Expr) (string, bool) {
	var (
		tokFile = pass.Fset.File(e.Pos())
		lo, hi  = tokFile.Offset(e.Pos()), tokFile.Offset(e.End())
	)
	if lo < 0 || hi < lo || hi > len(src) {
		return "", false
	}
	return string(src[lo:hi]), true
}
