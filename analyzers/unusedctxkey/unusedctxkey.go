// Package unusedctxkey provides an analysis.Analyzer that reports a context
// key constant with no use outside a Context.Value lookup.
//
// The Context.Value doc says that a function which stores a value in a Context
// "typically allocate[s] a key in a global variable then use[s] that key as
// the argument to context.WithValue and Context.Value". A constant with no
// reference outside a Value key has no store under its name in the package:
// the call that would use it is missing.
//
// The rule reads the key type from the key argument of a context.WithValue or
// Context.Value call in the package. A constant of one of those types is
// reported when its only references are Context.Value keys. Any other
// reference, such as a WithValue key, a store or a comparison, leaves the
// constant alone. A constant anywhere inside a Value key expression counts as
// a Value key reference. A type that appears in neither call is not known to
// be a key type, so its constants are left alone. So are constants of a
// built-in type, whose type is shared with every other value of that kind, and
// package-scope exported constants in an importable package, which another
// package can pass to WithValue.
//
// A Value method stored in a variable is not followed, so its key argument
// counts as an ordinary use. Context compares keys by value, so a reported
// constant can read a value that another constant of equal value stores; the
// report concerns the name.
//
// No fix is offered: the edit could either delete the constant or add the
// store that gives it a value, and the tool cannot choose between them.
package unusedctxkey

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedctxkey",
	Doc:  "reports a context key constant with no use outside a Value lookup",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		keyTypes = findKeyTypes(pass)
		valueIDs = findValueKeys(pass)
		read     = usedConsts(pass, valueIDs)
	)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			gen, ok := n.(*ast.GenDecl)
			if ok && gen.Tok == token.CONST {
				check(pass, gen, keyTypes, read)
			}
			return true
		})
	}
	return nil, nil
}

// findKeyTypes returns the named types used as a context key in the package.
func findKeyTypes(pass *analysis.Pass) map[types.Type]struct{} {
	out := make(map[types.Type]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch {
			case isWithValue(pass, call):
				addType(pass, out, call.Args[1])
			default:
				if key, ok := valueKey(pass, call); ok {
					addType(pass, out, key)
				}
			}
			return true
		})
	}
	return out
}

// findValueKeys returns the identifiers that name a constant in a Value key
// expression.
func findValueKeys(pass *analysis.Pass) map[*ast.Ident]struct{} {
	out := make(map[*ast.Ident]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if key, ok := valueKey(pass, call); ok {
				for _, id := range consts(pass, key) {
					out[id] = struct{}{}
				}
			}
			return true
		})
	}
	return out
}

// usedConsts returns the constants the package reads outside a Value key.
func usedConsts(pass *analysis.Pass, valueIDs map[*ast.Ident]struct{}) map[types.Object]struct{} {
	out := make(map[types.Object]struct{})
	for id, obj := range pass.TypesInfo.Uses {
		if _, ok := obj.(*types.Const); !ok {
			continue
		}
		if _, ok := valueIDs[id]; ok {
			continue
		}
		out[obj] = struct{}{}
	}
	return out
}

// check reports the constants under gen whose type is a context key type and
// that the package reads nowhere outside a Value key.
func check(pass *analysis.Pass, gen *ast.GenDecl, keyTypes map[types.Type]struct{}, read map[types.Object]struct{}) {
	for _, spec := range gen.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, id := range value.Names {
			obj, ok := pass.TypesInfo.Defs[id].(*types.Const)
			if !ok || id.Name == "_" {
				continue
			}
			// Only a package-scope name in an importable package can reach
			// another package.
			pkgScope := obj.Parent() == pass.Pkg.Scope()
			if pkgScope && obj.Exported() && pass.Pkg.Name() != "main" {
				continue
			}
			if !isKeyType(keyTypes, obj.Type()) {
				continue
			}
			if _, used := read[obj]; used {
				continue
			}
			msg := "context key " + id.Name +
				" is never used with context.WithValue"
			pass.Reportf(id.Pos(), "%s", msg)
		}
	}
}

// addType records the named type key has, if it has one.
func addType(pass *analysis.Pass, out map[types.Type]struct{}, key ast.Expr) {
	t := types.Unalias(pass.TypesInfo.TypeOf(key))
	if named, ok := t.(*types.Named); ok {
		out[named] = struct{}{}
	}
}

// consts returns the identifiers in e that name a constant.
func consts(pass *analysis.Pass, e ast.Expr) (ids []*ast.Ident) {
	ast.Inspect(e, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := pass.TypesInfo.Uses[id].(*types.Const); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

// isKeyType reports whether t is one of the named types keyTypes holds.
func isKeyType(keyTypes map[types.Type]struct{}, t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	_, ok = keyTypes[named]
	return ok
}

// isWithValue reports whether call is context.WithValue, arriving as a
// selector or, through a dot import, as a bare identifier.
func isWithValue(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 3 {
		return false
	}
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

// valueKey returns the key argument of a Context.Value call, as a method call
// or a method expression.
func valueKey(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	obj := pass.TypesInfo.Uses[sel.Sel]
	if obj == nil {
		return nil, false
	}
	pkg := obj.Pkg()
	if pkg == nil || pkg.Path() != "context" || obj.Name() != "Value" {
		return nil, false
	}
	if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.IsType() {
		if len(call.Args) == 2 {
			return call.Args[1], true
		}
		return nil, false
	}
	if len(call.Args) == 1 {
		return call.Args[0], true
	}
	return nil, false
}
