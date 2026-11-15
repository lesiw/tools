package evalorder

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// reads returns the places expr reads. The body of a function literal is left
// to the return inside it.
func reads(pass *analysis.Pass, expr ast.Expr) (out []place) {
	ast.Inspect(expr, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if addr, ok := e.(*ast.UnaryExpr); ok && addr.Op == token.AND {
			// Taking an address is not a read of the operand, but a pointer on
			// the way to it and an index inside it are read.
			out = append(out, reached(pass, addr.X)...)
			for _, index := range indices(addr.X) {
				out = append(out, reads(pass, index)...)
			}
			return false
		}
		if receiver := taken(pass, e); receiver != nil {
			// The receiver's address is taken, not its value, but a pointer on
			// the way to it and an index inside it are read.
			out = append(out, reached(pass, receiver)...)
			for _, index := range indices(receiver) {
				out = append(out, reads(pass, index)...)
			}
			return false
		}
		if slice, ok := e.(*ast.SliceExpr); ok {
			if isArray(pass.TypesInfo.TypeOf(slice.X)) {
				// Slicing an array takes its address, and its bounds are
				// read.
				bounds := []ast.Expr{slice.Low, slice.High, slice.Max}
				for _, bound := range bounds {
					if bound != nil {
						out = append(out, reads(pass, bound)...)
					}
				}
				return false
			}
		}
		p, ok := placeOf(pass, e)
		if !ok {
			return true
		}
		out = append(out, p)
		for _, index := range indices(e) {
			out = append(out, reads(pass, index)...)
		}
		return false // the chain is one read, at its top
	})
	return out
}

// addresses returns the places expr hands to a callee by address. Only a
// pointer parameter counts: an address passed as an interface is how fmt and
// encoding/json read a value.
func addresses(pass *analysis.Pass, expr ast.Expr) (out []place) {
	eval(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sig, ok := signature(pass, call)
		if !ok {
			return true
		}
		for i, arg := range call.Args {
			if !pointerParam(sig, i) {
				continue
			}
			if operand := addressed(pass, arg); operand != nil {
				if p, ok := placeOf(pass, operand); ok {
					out = append(out, p)
				}
			}
		}
		return true
	})
	return out
}

// taken returns the receiver whose address e takes, and nil when e takes none.
// A method value copies its receiver, and a pointer receiver on a value takes
// the address instead.
func taken(pass *analysis.Pass, e ast.Expr) ast.Expr {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return nil
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return nil
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	if !isPointer(sig.Recv().Type()) {
		return nil
	}
	if isPointer(pass.TypesInfo.TypeOf(sel.X)) {
		return nil
	}
	return sel.X
}

// addressed returns the expression whose address expr takes, and nil when expr
// takes none. A conversion and a parenthesis do not change the address they
// wrap.
func addressed(pass *analysis.Pass, expr ast.Expr) ast.Expr {
	for {
		switch e := ast.Unparen(expr).(type) {
		case *ast.UnaryExpr:
			if e.Op != token.AND {
				return nil
			}
			return e.X
		case *ast.CallExpr:
			if !pass.TypesInfo.Types[e.Fun].IsType() {
				return nil
			}
			expr = e.Args[0]
		default:
			return nil
		}
	}
}

// signature returns the signature of the callee of call.
func signature(pass *analysis.Pass, call *ast.CallExpr) (*types.Signature, bool) {
	if pass.TypesInfo.Types[call.Fun].IsType() {
		return nil, false // a conversion names a type
	}
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		// The type checker writes a signature of its own for a builtin.
		if _, ok := pass.TypesInfo.Uses[id].(*types.Builtin); ok {
			return nil, false
		}
	}
	return signatureOf(pass.TypesInfo.TypeOf(call.Fun))
}

// signatureOf returns the signature that every type t can hold has.
func signatureOf(t types.Type) (*types.Signature, bool) {
	if t == nil {
		return nil, false
	}
	if sig, ok := t.Underlying().(*types.Signature); ok {
		return sig, true
	}
	param, ok := t.(*types.TypeParam)
	if !ok {
		return nil, false
	}
	iface, ok := param.Constraint().Underlying().(*types.Interface)
	if !ok {
		return nil, false
	}
	var sig *types.Signature
	var terms int
	for e := range iface.EmbeddedTypes() {
		union, ok := e.(*types.Union)
		if !ok {
			union = types.NewUnion([]*types.Term{types.NewTerm(false, e)})
		}
		for term := range union.Terms() {
			next, ok := term.Type().Underlying().(*types.Signature)
			if !ok || (sig != nil && !types.Identical(sig, next)) {
				return nil, false
			}
			sig, terms = next, terms+1
		}
	}
	return sig, sig != nil && terms > 0
}

// pointerParam reports whether the parameter for argument i of a call with
// signature sig is a pointer.
func pointerParam(sig *types.Signature, i int) bool {
	params := sig.Params()
	if sig.Variadic() && i >= params.Len()-1 {
		// The last parameter of a variadic signature is a slice, and each
		// argument from here provides one of its elements.
		last := params.At(params.Len() - 1).Type()
		if slice, ok := last.Underlying().(*types.Slice); ok {
			last = slice.Elem()
		}
		return isPointer(last)
	}
	if i >= params.Len() {
		return false
	}
	return isPointer(params.At(i).Type())
}
