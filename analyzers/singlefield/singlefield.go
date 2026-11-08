// Package singlefield provides an analysis.Analyzer that reports
// single-field structs replaceable by their underlying type:
// type X struct{ s string } can be type X string.
//
// The fold — the type and every use, X{f: v} to X(v), x.f to
// T(x) — is the author's work. For an exported type it changes
// the API of every importer, so it is a release decision.
//
// A struct whose one field is an interface is exempt: the folded
// type would itself be an interface, not a concrete type. An
// embedded field and a field with a struct tag are exempt:
// promotion and serialization shape are load-bearing. A pointer
// field is exempt: a named pointer type cannot be a method
// receiver, so the folded type could never carry behavior.
//
// Type aliases and generic types are not reported.
package singlefield

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "singlefield",
	Doc:  "reports single-field structs replaceable by their underlying type",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if ok {
				checkSpec(pass, spec)
			}
			return true
		})
	}
	return nil, nil
}

// checkSpec reports spec when it declares a single-field struct no
// exemption covers.
func checkSpec(pass *analysis.Pass, spec *ast.TypeSpec) {
	if spec.Assign.IsValid() || spec.TypeParams != nil {
		return
	}
	st, ok := spec.Type.(*ast.StructType)
	if !ok || len(st.Fields.List) != 1 {
		return
	}
	field := st.Fields.List[0]
	if len(field.Names) != 1 || field.Tag != nil {
		return
	}
	typ := pass.TypesInfo.TypeOf(field.Type)
	if typ == nil || types.IsInterface(typ) {
		return
	}
	if _, isPtr := typ.Underlying().(*types.Pointer); isPtr {
		// A named pointer type cannot be a method receiver, so the
		// folded type could never carry behavior.
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos: spec.Pos(),
		End: spec.Type.End(),
		Message: "single-field struct can be replaced by its " +
			"underlying type: " + spec.Name.Name + " " +
			types.ExprString(field.Type),
	})
}
