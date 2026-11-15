// Package ctxkey provides an analysis.Analyzer that reports a
// context.WithValue key of a built-in type.
//
// The documentation for context.WithValue says the key "should not be of type
// string or any other built-in type to avoid collisions between packages
// using context", and that users "should define their own types for keys". A
// key whose type is built in, such as string, int, or bool, is the same type
// in every package, so two packages that pick the same value collide.
//
// The rule stands down once the key has a defined type, in this package or in
// another: two packages may declare a type of the same name, and the two types
// differ. An alias is the same type under another name, so a key of an alias
// to a string still collides and is still reported. The rule reads the type
// the argument has at the call; a type that is abstract there, an interface or
// a type parameter, is left alone, because the type it stands for is not
// visible. Neither is a nil key: WithValue panics on one, so no value is ever
// stored under it.
//
// The rule is adapted from staticcheck's SA1029. Upstream also reports a key
// of non-comparable type, which panics in WithValue, and a key of empty
// anonymous struct type, which is not a built-in type.
//
// No fix is offered: the edit would have to invent a key type and a name for
// it, and that name becomes part of the package's API.
package ctxkey

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ctxkey",
	Doc:  "reports a context.WithValue key of a built-in type",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 || !isWithValue(pass, call) {
				return true
			}
			check(pass, call.Args[1])
			return true
		})
	}
	return nil, nil
}

// isWithValue reports whether call is context.WithValue, arriving as a
// selector or, through a dot import, as a bare identifier.
func isWithValue(pass *analysis.Pass, call *ast.CallExpr) bool {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		id = fun.Sel
	case *ast.Ident:
		id = fun
	default:
		return false
	}
	obj := pass.TypesInfo.Uses[id]
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "context" &&
		obj.Name() == "WithValue"
}

// check reports key when it has a built-in type. WithValue panics on a nil
// key.
func check(pass *analysis.Pass, key ast.Expr) {
	var (
		t         = pass.TypesInfo.TypeOf(key)
		basic, ok = types.Unalias(t).(*types.Basic)
	)
	if !ok || basic.Kind() == types.UntypedNil {
		return
	}
	name := types.TypeString(basic, nil)
	// The report prints a position, not the key expression, so it names the
	// alias too.
	if alias, ok := t.(*types.Alias); ok {
		via := types.TypeString(alias, types.RelativeTo(pass.Pkg))
		name += " (alias " + via + ")"
	}
	pass.Reportf(key.Pos(), "context key has the built-in type %s; define a key type", name)
}
