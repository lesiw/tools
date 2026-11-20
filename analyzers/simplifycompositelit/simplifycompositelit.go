// Package simplifycompositelit provides an analysis.Analyzer that reports an
// element or map key that repeats the type its composite literal names.
//
// Vendored from gopls' simplifycompositelit
// (gopls/internal/analysis/simplifycompositelit, golang.org/x/tools/gopls
// v0.24.0, BSD-3-Clause, Copyright 2020 The Go Authors), which is internal and
// cannot be imported. The rule and its diagnostic messages are upstream's; the
// code is adapted to this repository's helpers.
//
// The spec allows the elision: within a composite literal of array, slice, or
// map type T, "elements or map keys that are themselves composite literals may
// elide the respective literal type if it is identical to the element or key
// type of T", and an element or key whose type is *T may elide the & of an
// address as well (https://go.dev/ref/spec#Composite_literals). gofmt -s writes
// the same simplification
// (https://pkg.go.dev/cmd/gofmt#hdr-The_simplify_command), which vet and fix do
// not run: a tree already formatted with it has nothing for this rule.
//
// The outer literal's type is read from its syntax, so a literal written
// through a name is left alone. The fix is withheld when the edit would delete
// a comment, where upstream offers it unconditionally.
package simplifycompositelit

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "simplifycompositelit",
	Doc:  "reports an element or map key that repeats the type its composite literal names",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.CompositeLit); ok {
				check(pass, file, lit)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, file *ast.File, lit *ast.CompositeLit) {
	var (
		kind             string
		keyType, eltType ast.Expr
	)
	switch typ := lit.Type.(type) {
	case *ast.ArrayType:
		eltType = typ.Elt
		if typ.Len != nil {
			kind = "array"
		} else {
			kind = "slice"
		}
	case *ast.MapType:
		keyType = typ.Key
		eltType = typ.Value
		kind = "map"
	default:
		// A struct literal, a named type, or no type at all leaves the
		// elements nothing to elide against.
		return
	}
	for _, elt := range lit.Elts {
		if kve, ok := elt.(*ast.KeyValueExpr); ok {
			if keyType != nil {
				simplify(pass, file, keyType, kve.Key, kind)
			}
			elt = kve.Value
		}
		simplify(pass, file, eltType, elt, kind)
	}
}

// simplify reports expr when it spells out the type want denotes.
func simplify(pass *analysis.Pass, file *ast.File, want, expr ast.Expr, kind string) {
	if lit, ok := expr.(*ast.CompositeLit); ok && lit.Type != nil {
		if sameType(pass, want, lit.Type) {
			report(pass, file, token.NoPos, lit.Type, kind)
		}
	}
	// An element of a pointer type *T written as &T{...} may omit the
	// address as well as the type.
	star, ok := want.(*ast.StarExpr)
	if !ok {
		return
	}
	addr, ok := expr.(*ast.UnaryExpr)
	if !ok || addr.Op != token.AND {
		return
	}
	lit, ok := addr.X.(*ast.CompositeLit)
	if !ok || lit.Type == nil {
		return
	}
	if sameType(pass, star.X, lit.Type) {
		report(pass, file, addr.OpPos, lit.Type, kind)
	}
}

// sameType reports whether a and b name the same type, through an alias on
// either side.
func sameType(pass *analysis.Pass, a, b ast.Expr) bool {
	return types.Identical(pass.TypesInfo.TypeOf(a), pass.TypesInfo.TypeOf(b))
}

// report reports the redundant type inner of a kind literal.
func report(pass *analysis.Pass, file *ast.File, ampPos token.Pos, inner ast.Expr, kind string) {
	var (
		start = inner.Pos()
		end   = inner.End()
		amp   string
		edits []analysis.TextEdit
	)
	if ampPos.IsValid() {
		amp = "&"
		start = ampPos
		edits = append(edits, analysis.TextEdit{Pos: ampPos, End: ampPos + 1})
	}
	edits = append(edits, analysis.TextEdit{Pos: inner.Pos(), End: end})
	d := analysis.Diagnostic{
		Pos:     start,
		End:     end,
		Message: fmt.Sprintf("redundant type in %s literal", kind),
	}
	if !astexpr.Commented(file, inner.Pos(), end) {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove '" + amp + astexpr.Text(inner) + "'",
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}
