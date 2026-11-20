// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package embeddirective provides an analysis.Analyzer that reports a file
// with a //go:embed directive and no embed import, and a directive that does
// not precede one variable of type string, []byte or embed.FS with no value.
//
// The embed package says the declaration is "a single variable" and its type
// "must be a string type, or a slice of a byte type, or FS (or an alias of
// FS)". The rule reads the slice case as []byte exactly, as upstream does, so
// a slice of a defined byte type is reported even though the embed package's
// doc allows it.
//
// The missing import and a constant initializer each get a fix. The other
// shapes are reported without one: each needs a move, a name, or a type the
// analyzer cannot choose, or would delete a comment or change behavior.
//
// Vendored from gopls' embed (gopls/internal/analysis/embeddirective,
// golang.org/x/tools/gopls v0.24.0), which is internal and cannot be imported:
// the rule and the code are copied, the import fix is written against this
// repository's internal/imports, and the fix that drops an initializer is
// added here. Upstream names the analyzer embed.
package embeddirective

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "embeddirective",
	Doc:  "reports a file that does not import embed, and a directive with a bad declaration",
	Run:  run,
	// A rejected directive stops the build, and the loader marks the
	// package ill-typed because of it, so the analyzer runs despite that.
	RunDespiteErrors: true,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		check(pass, file)
	}
	return nil, nil
}

func check(pass *analysis.Pass, file *ast.File) {
	var (
		directives = embedDirectives(file)
		imported   = importsEmbed(file)
	)
	for _, c := range directives {
		var (
			pos = c.Pos()
			end = pos + token.Pos(len("//go:embed"))
		)
		if !imported {
			pass.Report(analysis.Diagnostic{
				Pos:     pos,
				End:     end,
				Message: `must import "embed" when using go:embed directives`,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   `Add the missing "embed" import`,
					TextEdits: embedImport(pass, file),
				}},
			})
		}
		var msg string
		var edits []analysis.TextEdit
		spec := nextVarSpec(c, file)
		switch {
		case spec == nil:
			msg = `go:embed directives must precede a "var" declaration`
		case len(spec.Names) != 1:
			msg = "declarations following go:embed directives must define a single variable"
		case len(spec.Values) > 0:
			msg = "declarations following go:embed directives must not specify a value"
			edits = dropValue(pass, file, spec)
		case !embeddable(pass.TypesInfo.Defs[spec.Names[0]]):
			msg = "declarations following go:embed directives must be of type string, []byte or embed.FS"
		}
		if msg == "" {
			continue
		}
		diagnostic := analysis.Diagnostic{Pos: pos, End: end, Message: msg}
		if len(edits) > 0 {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message:   "Drop the value",
				TextEdits: edits,
			}}
		}
		pass.Report(diagnostic)
	}
}

// embedDirectives returns the comments in file that carry a //go:embed
// directive.
func embedDirectives(file *ast.File) (out []*ast.Comment) {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:embed ") {
				out = append(out, c)
			}
		}
	}
	return out
}

// embedImport returns the edits that add embed. A file that names the package,
// as one declaring an embed.FS does, needs the name: the blank import leaves
// it undefined, and goimports then adds a second import of the same path.
func embedImport(pass *analysis.Pass, file *ast.File) []analysis.TextEdit {
	if !namesEmbed(pass, file) {
		return []analysis.TextEdit{imports.AddBlank(pass, file, "embed")}
	}
	return []analysis.TextEdit{imports.Add(pass, file, "embed")}
}

// namesEmbed reports whether file uses embed as a qualifier that nothing
// declares, which is what a missing import leaves.
func namesEmbed(pass *analysis.Pass, file *ast.File) bool {
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "embed" {
				found = pass.TypesInfo.Uses[id] == nil
			}
		}
		return true
	})
	return found
}

// importsEmbed reports whether file imports embed, under any name.
func importsEmbed(file *ast.File) bool {
	for _, spec := range file.Imports {
		if spec.Path.Value == `"embed"` {
			return true
		}
	}
	return false
}

// nextVarSpec returns the ValueSpec of the variable declaration that follows
// c, or nil when the next declaration is not a variable declaration.
func nextVarSpec(c *ast.Comment, file *ast.File) *ast.ValueSpec {
	var next ast.Decl
	for _, decl := range file.Decls {
		if c.End() < decl.End() {
			next = decl
			break
		}
	}
	if next == nil || next.Pos() == token.NoPos {
		return nil
	}
	decl, ok := next.(*ast.GenDecl)
	if !ok || decl.Tok != token.VAR {
		return nil
	}
	// A var block can hold several specs, and the one the directive names is
	// not always the first.
	var nextSpec ast.Spec
	for _, spec := range decl.Specs {
		if c.End() < spec.End() {
			nextSpec = spec
			break
		}
	}
	spec, ok := nextSpec.(*ast.ValueSpec)
	if !ok {
		return nil
	}
	return spec
}

// embeddable reports whether obj's type can hold the contents of a //go:embed
// directive.
func embeddable(obj types.Object) bool {
	if obj == nil {
		return false
	}
	t := obj.Type()
	// For embed.FS the underlying type is an implementation detail, so a
	// name that resolves to it is enough.
	if named, ok := types.Unalias(t).(*types.Named); ok {
		var (
			decl = named.Obj()
			pkg  = decl.Pkg()
		)
		if pkg != nil && pkg.Path() == "embed" && decl.Name() == "FS" {
			return true
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return types.Identical(u, types.Typ[types.String])
	case *types.Slice:
		return types.Identical(u.Elem(), types.Typ[types.Byte])
	}
	return false
}

// dropValue returns the edit that removes the value from spec, or nil when no
// safe edit exists.
func dropValue(pass *analysis.Pass, file *ast.File, spec *ast.ValueSpec) []analysis.TextEdit {
	// A declaration with no type would declare nothing after the value goes.
	if spec.Type == nil {
		return nil
	}
	// A value list the type checker rejected would leave a value behind.
	if len(spec.Values) != 1 {
		return nil
	}
	// Dropping the value of a type the directive cannot fill only moves the
	// report to the type.
	if !embeddable(pass.TypesInfo.Defs[spec.Names[0]]) {
		return nil
	}
	// A value that is not a constant expression may do something when it is
	// evaluated, so the edit leaves it in place.
	if pass.TypesInfo.Types[spec.Values[0]].Value == nil {
		return nil
	}
	// The edit would take any comment between the type and the value with it.
	if commented(file, spec.Type.End(), spec.Values[0].End()) {
		return nil
	}
	return []analysis.TextEdit{{
		Pos: spec.Type.End(),
		End: spec.Values[0].End(),
	}}
}

// commented reports whether a comment overlaps the span from lo to hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
