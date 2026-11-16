// Package ineffectualcall provides an analysis.Analyzer that reports a call
// written as a statement to a method with a value receiver whose body only
// returns an effect-free value.
//
// A method declaration gives the receiver an extra parameter section
// (https://go.dev/ref/spec#Method_declarations), and a call assigns the
// receiver expression to that parameter (https://go.dev/ref/spec#Calls). A
// value receiver is a copy. When the method's body is one return of an
// effect-free value and the call is a statement, the copy and the result are
// discarded, so the call has no effect.
//
// The rule reports a call to a method declared with a value receiver, on a
// value. A pointer receiver is left alone. A call through a pointer is left
// alone because the implicit dereference can panic on nil. A call whose result
// is used is not an expression statement, so it is left alone too. The
// receiver, every argument, and every returned expression also have to be
// effect-free, or the call can do work or panic. The README catalogs every
// check and exemption.
//
// Report only: deleting the call can leave a variable or an import unused,
// which breaks the build, or remove a comment with the statement.
//
// The rule is adapted from staticcheck's SA4017, which reports a discarded
// result when the function has no effect other than the result.
package ineffectualcall

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ineffectualcall",
	Doc:  "reports a call statement to a value-receiver method that only returns an effect-free value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	decls := methods(pass)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := valueCall(pass, call)
			if !ok {
				return true
			}
			decl := decls[fn]
			if decl == nil || !onlyValue(pass, decl.Body) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "call to " + fn.Name() + " has no effect",
			})
			return true
		})
	}
	return nil, nil
}

// methods returns the method declarations in the package, by the function
// object each one declares.
func methods(pass *analysis.Pass) map[*types.Func]*ast.FuncDecl {
	out := make(map[*types.Func]*ast.FuncDecl)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			if obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func); ok {
				out[obj] = fn
			}
		}
	}
	return out
}

// valueCall returns the method a call selects, when it calls a value-receiver
// method on a value that yields a result.
func valueCall(pass *analysis.Pass, call *ast.CallExpr) (*types.Func, bool) {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return nil, false
	}
	if selection.Indirect() || pointer(selection.Recv()) {
		return nil, false
	}
	if !value(pass, selector.X) {
		return nil, false
	}
	for _, arg := range call.Args {
		if !value(pass, arg) {
			return nil, false
		}
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return nil, false
	}
	fn = fn.Origin()
	signature, ok := fn.Type().(*types.Signature)
	if !ok || signature.Recv() == nil || pointer(signature.Recv().Type()) {
		return nil, false
	}
	if signature.Results().Len() == 0 {
		return nil, false
	}
	return fn, true
}

// pointer reports whether t is a pointer type.
func pointer(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Pointer)
	return ok
}

// onlyValue reports whether body only returns values, and evaluates no
// expression that can have an effect or panic.
func onlyValue(pass *analysis.Pass, body *ast.BlockStmt) bool {
	if body == nil || len(body.List) != 1 {
		return false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	for _, result := range ret.Results {
		if !value(pass, result) {
			return false
		}
	}
	return true
}

// value reports whether expr evaluates to a value without an effect or a
// panic. Constant expressions are accepted first. Non-constant calls are left
// out, except a conversion that cannot panic.
func value(pass *analysis.Pass, expr ast.Expr) bool {
	if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.Value != nil {
		return true
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.ParenExpr:
		return value(pass, expr.X)
	case *ast.SelectorExpr:
		return field(pass, expr)
	case *ast.UnaryExpr:
		switch expr.Op {
		case token.ADD, token.SUB, token.XOR, token.NOT, token.AND:
			return value(pass, expr.X)
		}
	case *ast.BinaryExpr:
		return basicBinary(pass, expr) &&
			value(pass, expr.X) && value(pass, expr.Y)
	case *ast.CompositeLit:
		return literal(pass, expr)
	case *ast.FuncLit:
		return true
	case *ast.CallExpr:
		return conversion(pass, expr) && value(pass, expr.Args[0])
	}
	return false
}

// field reports whether selector is an effect-free field or package-level
// value.
func field(pass *analysis.Pass, selector *ast.SelectorExpr) bool {
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil {
		// A selector without a selection is package-qualified.
		switch pass.TypesInfo.Uses[selector.Sel].(type) {
		case *types.Var, *types.Const, *types.Func:
			return true
		}
		return false
	}
	if selection.Kind() == types.FieldVal {
		return !selection.Indirect() &&
			!pointer(pass.TypesInfo.TypeOf(selector.X)) &&
			value(pass, selector.X)
	}
	return false
}

// basicBinary reports whether binary is an operation on basic values that
// cannot panic.
func basicBinary(pass *analysis.Pass, binary *ast.BinaryExpr) bool {
	switch binary.Op {
	case token.ADD, token.SUB, token.MUL,
		token.AND, token.OR, token.XOR, token.AND_NOT,
		token.LAND, token.LOR,
		token.EQL, token.NEQ,
		token.LSS, token.LEQ, token.GTR, token.GEQ:
	default:
		return false
	}
	return basic(pass.TypesInfo.TypeOf(binary.X)) &&
		basic(pass.TypesInfo.TypeOf(binary.Y))
}

// basic reports whether t has a basic underlying type.
func basic(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok
}

// literal reports whether lit builds a value without an effect or a panic. A
// map literal can panic on an unhashable key, so it is left out. A literal of
// a type parameter is left out because its type set can hold a map.
func literal(pass *analysis.Pass, lit *ast.CompositeLit) bool {
	if t := pass.TypesInfo.TypeOf(lit); t != nil {
		t = types.Unalias(t)
		if _, ok := t.(*types.TypeParam); ok {
			return false
		}
		if _, ok := t.Underlying().(*types.Map); ok {
			return false
		}
	}
	for _, elt := range lit.Elts {
		if pair, ok := elt.(*ast.KeyValueExpr); ok {
			elt = pair.Value
		}
		if !value(pass, elt) {
			return false
		}
	}
	return true
}

// conversion reports whether call converts one value to a type without a
// panic. A conversion to an array or a pointer to an array is left out,
// because a conversion from a slice can panic. A conversion to a type
// parameter is left out because its type set can hold an array.
func conversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return false
	}
	targetType := types.Unalias(tv.Type)
	if _, ok := targetType.(*types.TypeParam); ok {
		return false
	}
	target := targetType.Underlying()
	if _, ok := target.(*types.Array); ok {
		return false
	}
	if ptr, ok := target.(*types.Pointer); ok {
		if _, ok := types.Unalias(ptr.Elem()).Underlying().(*types.Array); ok {
			return false
		}
	}
	return true
}
