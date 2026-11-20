// Package maprange provides an analysis.Analyzer that reports a call to
// maps.Keys or maps.Values in a range.
//
// Vendored from gopls' maprange
// (gopls/internal/analysis/maprange, golang.org/x/tools/gopls v0.24.0), which
// is internal and cannot be imported. The rule and its messages are
// upstream's; the code is adapted to this repository's helpers. gopls is
// BSD-3-Clause, Copyright 2025 The Go Authors.
//
// Upstream drops the call from a loop that binds the index and a blank value,
// as in for i, _ := range maps.Keys(m), which turns the index into a key; this
// rule writes len(m) instead, which keeps the index. It withholds some fixes
// upstream offers, which the README lists along with the shapes the rule
// leaves alone.
//
// The maps package documents Keys as an iterator over the keys in m and Values
// as an iterator over its values (https://pkg.go.dev/maps). The range clause
// yields the key and then the value for a map
// (https://go.dev/ref/spec#For_range), so the call is redundant.
package maprange

import (
	"go/ast"
	"go/token"
	"go/types"
	"go/version"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "maprange",
	Doc:  "reports a call to maps.Keys or maps.Values in a range",
	Run:  run,
}

// xmaps is golang.org/x/exp/maps, whose Keys and Values return slices.
const xmaps = "golang.org/x/exp/maps"

// call is a call to maps.Keys or maps.Values, named through its package or a
// dot import.
type call struct {
	expr      *ast.CallExpr
	pkg, fn   string
	qualified bool
}

// loop is a range clause over a call to maps.Keys or maps.Values.
type loop struct {
	call
	stmt     *ast.RangeStmt
	key      bool // the key is set and not blank
	value    bool // the value is set and not blank
	counting bool // an x/exp loop that counts entries
}

func run(pass *analysis.Pass) (any, error) {
	switch pass.Pkg.Path() {
	case "maps", xmaps:
		return nil, nil // These packages know how to use their own APIs.
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.RangeStmt); ok {
				report(pass, file, stmt)
			}
			return true
		})
	}
	return nil, nil
}

// mapsCall returns e as a call to maps.Keys or maps.Values, whether the
// function is named through its package or a dot import.
func mapsCall(pass *analysis.Pass, e ast.Expr) (call, bool) {
	expr, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(expr.Args) != 1 {
		return call{}, false
	}
	_, fn, ok := uses.CalleeOf(pass, expr.Fun)
	if !ok || fn.Pkg() == nil {
		return call{}, false
	}
	name := fn.Name()
	if name != "Keys" && name != "Values" {
		return call{}, false
	}
	pkg := fn.Pkg().Path()
	if pkg != "maps" && pkg != xmaps {
		return call{}, false
	}
	return call{
		expr:      expr,
		pkg:       pkg,
		fn:        name,
		qualified: throughPackage(expr.Fun),
	}, true
}

// throughPackage reports whether fun names the function through its package
// rather than through a dot import.
func throughPackage(fun ast.Expr) bool {
	switch x := ast.Unparen(fun).(type) {
	case *ast.IndexExpr:
		return throughPackage(x.X)
	case *ast.IndexListExpr:
		return throughPackage(x.X)
	case *ast.SelectorExpr:
		return true
	}
	return false
}

// report diagnoses stmt when it ranges over the result of a call to maps.Keys
// or maps.Values.
func report(pass *analysis.Pass, file *ast.File, stmt *ast.RangeStmt) {
	c, ok := mapsCall(pass, stmt.X)
	if !ok {
		return
	}
	l := loop{
		call:  c,
		stmt:  stmt,
		key:   !isBlank(stmt.Key),
		value: !isBlank(stmt.Value),
	}
	if l.key && l.value {
		// Only a slice binds both variables, and no edit keeps the loop's
		// meaning.
		return
	}
	// Ranging a slice yields an index and an element, so a loop that names
	// only the first of those counts entries.
	l.counting = c.pkg == xmaps && l.key && !l.value
	name := c.pkg + "." + c.fn
	if l.counting && !rangesInt(pass, file) {
		pass.Report(analysis.Diagnostic{
			Pos:     c.expr.Pos(),
			End:     c.expr.End(),
			Message: "likely incorrect use of " + name + " (returns a slice)",
		})
		return
	}
	d := analysis.Diagnostic{
		Pos:     c.expr.Pos(),
		End:     c.expr.End(),
		Message: "unnecessary and inefficient call of " + name,
	}
	if edits, ok := fix(pass, file, l); ok {
		message := "Remove unnecessary call to " + name
		if l.counting {
			message = "Replace " + name + " with len(" +
				astexpr.Text(c.expr.Args[0]) + ")"
		}
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   message,
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// fix returns the edits that drop the call, and reports false where the
// diagnostic carries none.
func fix(pass *analysis.Pass, file *ast.File, l loop) ([]analysis.TextEdit, bool) {
	if !l.qualified {
		// Removing the call can strand the dot import it was named
		// through, and goimports keeps an unused one.
		return nil, false
	}
	if l.pkg == xmaps && !l.counting {
		// The slice is a copy, so ranging the map instead iterates a
		// different set of entries.
		return nil, false
	}
	if l.counting && !builtinLen(pass, l.stmt.Pos()) {
		// A local len would take the call the fix writes.
		return nil, false
	}
	edits := rewrite(l)
	for _, edit := range edits {
		if commented(file, edit.Pos, edit.End) {
			return nil, false // the edit would delete the comment
		}
	}
	return edits, true
}

// rewrite returns the edits that drop the call, leaving a range over the map
// for the keys or values it yields, or over len(m) for a loop that only counts
// entries.
func rewrite(l loop) (edits []analysis.TextEdit) {
	// for i, _ := range maps.Keys(m) declares a second variable a count has
	// no use for.
	if l.counting && l.stmt.Value != nil {
		edits = append(edits, analysis.TextEdit{
			Pos: l.stmt.Key.End(),
			End: l.stmt.Value.End(),
		})
	}
	// for v := range maps.Values(m) binds the value, but a range over a map
	// binds the key first.
	if l.pkg == "maps" && l.fn == "Values" && l.key {
		edits = append(edits, analysis.TextEdit{
			Pos:     l.stmt.Key.Pos(),
			End:     l.stmt.Key.Pos(),
			NewText: []byte("_, "),
		})
	}
	var replacement string
	if l.counting {
		// len(m) is the count the slice of keys or values stood for.
		replacement = "len"
	}
	return append(edits, analysis.TextEdit{
		Pos:     l.expr.Fun.Pos(),
		End:     l.expr.Fun.End(),
		NewText: []byte(replacement),
	})
}

// commented reports whether a comment lies in [pos, end), the span an edit
// would delete.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}

// builtinLen reports whether len names the builtin at pos.
func builtinLen(pass *analysis.Pass, pos token.Pos) bool {
	_, obj := pass.Pkg.Scope().Innermost(pos).LookupParent("len", pos)
	return obj == types.Universe.Lookup("len")
}

// isBlank reports whether e is absent or the blank identifier.
func isBlank(e ast.Expr) bool {
	if e == nil {
		return true
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// rangesInt reports whether the file is checked against Go 1.22, where a range
// statement may iterate an integer. A file with no version of its own is
// checked against the whole language, so the len(m) fix applies.
func rangesInt(pass *analysis.Pass, file *ast.File) bool {
	v := pass.TypesInfo.FileVersions[file]
	return v == "" || version.Compare(v, "go1.22") >= 0
}
