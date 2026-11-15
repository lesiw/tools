// Package unexportedreturn provides an analysis.Analyzer that reports an
// exported function or method whose result is an unexported type declared in
// the same package.
//
// An identifier is accessible from another package only when it is exported
// (https://go.dev/ref/spec#Exported_identifiers), so a caller cannot write the
// type the result is declared with. The README catalogs every check and
// exemption.
//
// Report only: the repair needs an exported name for the type.
//
// The rule is adapted from revive's unexported-return.
package unexportedreturn

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unexportedreturn",
	Doc:  "reports an exported function or method whose result is an unexported type declared in the same package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	// A package main cannot be imported.
	if pass.Pkg.Name() == "main" {
		return nil, nil
	}
	// An internal package is imported only from its own tree, and those
	// importers can edit it.
	if slices.Contains(strings.Split(pass.Pkg.Path(), "/"), "internal") {
		return nil, nil
	}
	for _, file := range pass.Files {
		name := pass.Fset.Position(file.Pos()).Filename
		// A test file's code is only reachable from the package's own tests.
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			check(pass, fn)
		}
	}
	return nil, nil
}

func check(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Type.Results == nil || !fn.Name.IsExported() {
		return
	}
	kind := "func"
	if fn.Recv != nil {
		recv := pass.TypesInfo.TypeOf(fn.Recv.List[0].Type)
		if unexportedNamed(recv) != nil {
			return
		}
		kind = "method"
	}
	for _, field := range fn.Type.Results.List {
		named := unexportedNamed(pass.TypesInfo.TypeOf(field.Type))
		if named == nil || types.IsInterface(named) {
			continue
		}
		expr := ast.Unparen(field.Type)
		pass.Reportf(
			expr.Pos(),
			"exported %s %s returns unexported type %s",
			kind, fn.Name.Name, types.ExprString(expr),
		)
	}
}

// unexportedNamed returns the unexported named type behind typ's pointers and
// unexported aliases, and nil when there is none.
func unexportedNamed(typ types.Type) *types.Named {
	for {
		switch t := typ.(type) {
		case *types.Pointer:
			typ = t.Elem()
		case *types.Alias:
			if t.Obj().Exported() {
				return nil
			}
			typ = t.Rhs()
		case *types.Named:
			if t.Obj().Exported() {
				return nil
			}
			return t
		default:
			return nil
		}
	}
}
