// Package mapkey provides an analysis.Analyzer that reports a map key that is
// unhashable.
//
// The compiler rejects a slice, a map, or a function as a key type. An
// interface key type accepts them, and the map panics when it hashes one. The
// README lists the keys the rule leaves alone.
//
// The rule reports without a fix: several hashable forms of a key exist, and
// none is the right one.
//
// The name comes from go-critic's mapKey, which reports a string key with a
// space at either end, or a repeated key.
package mapkey

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "mapkey",
	Doc:  "reports a map key that is unhashable",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CompositeLit:
				checkLiteral(pass, n)
			case *ast.IndexExpr:
				checkIndex(pass, n)
			case *ast.CallExpr:
				checkDelete(pass, n)
			}
			return true
		})
	}
	return nil, nil
}

func checkLiteral(pass *analysis.Pass, lit *ast.CompositeLit) {
	if !isMap(pass, lit) {
		return
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		checkKey(pass, kv.Key)
	}
}

func checkIndex(pass *analysis.Pass, ix *ast.IndexExpr) {
	// Without this, the type argument of an instantiation, T[P], reads as a
	// key.
	if pass.TypesInfo.Types[ix].IsType() {
		return
	}
	if !isMap(pass, ix.X) {
		return
	}
	checkKey(pass, ix.Index)
}

func checkDelete(pass *analysis.Pass, call *ast.CallExpr) {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || len(call.Args) != 2 {
		return
	}
	if pass.TypesInfo.Uses[ident] != types.Universe.Lookup("delete") {
		return
	}
	if !isMap(pass, call.Args[0]) {
		return
	}
	checkKey(pass, call.Args[1])
}

func checkKey(pass *analysis.Pass, key ast.Expr) {
	typ := hashedType(pass, key)
	if typ == nil || !unhashable(typ) {
		return
	}
	// Names a type from another package by its package name, so a report says
	// http.Header where the import path is net/http.
	name := types.TypeString(typ, func(p *types.Package) string {
		if p == pass.Pkg {
			return ""
		}
		return p.Name()
	})
	pass.Reportf(key.Pos(), "map key of type %s is unhashable", name)
}

// hashedType returns the static type of the value the map hashes as key. A
// conversion to an interface hashes its operand, so the operand's type is the
// one a report names.
func hashedType(pass *analysis.Pass, key ast.Expr) types.Type {
	typ := pass.TypesInfo.TypeOf(key)
	if typ == nil {
		return nil
	}
	call, ok := ast.Unparen(key).(*ast.CallExpr)
	// A call returning an interface is not a conversion to one.
	if !ok || !pass.TypesInfo.Types[call.Fun].IsType() {
		return typ
	}
	if !types.IsInterface(typ) {
		return typ
	}
	// A type parameter's underlying type is its constraint, an interface, so
	// the conversion would be read through to its operand. The rule does not
	// read the type set, so the key is left alone.
	if _, isParam := types.Unalias(typ).(*types.TypeParam); isParam {
		return typ
	}
	return hashedType(pass, call.Args[0])
}

// unhashable reports whether no value of t can be hashed. An interface is
// never unhashable, since the hash is the value's, and neither is a type
// parameter, whose type set the rule does not read.
func unhashable(t types.Type) bool {
	switch t := t.Underlying().(type) {
	case *types.Slice, *types.Map, *types.Signature:
		return true
	case *types.Array:
		return unhashable(t.Elem())
	case *types.Struct:
		for field := range t.Fields() {
			if unhashable(field.Type()) {
				return true
			}
		}
	}
	return false
}

func isMap(pass *analysis.Pass, e ast.Expr) bool {
	typ := pass.TypesInfo.TypeOf(e)
	if typ == nil {
		return false
	}
	_, ok := typ.Underlying().(*types.Map)
	return ok
}
