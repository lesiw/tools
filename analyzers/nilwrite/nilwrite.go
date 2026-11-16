// Package nilwrite provides an analysis.Analyzer that reports a Write method
// that may modify the byte slice it was given.
//
// io.Writer's doc says "Write must not modify the slice data, even
// temporarily" (https://pkg.go.dev/io#Writer). A caller may reuse the slice
// after Write returns, so a method that stores into p[0] or appends to p may
// change data the caller still reads.
//
// The rule reports a write in a method named Write whose signature is
// io.Writer's. It reports an index assignment, append, copy, or clear that
// writes into the parameter or a slice of it. An append whose first argument
// is a three-index slice whose high and max bounds are equal constants or the
// same side-effect-free expression is left alone, and the rule does not follow
// the parameter through another variable or into another function.
//
// Report only: copying p changes where the bytes go, and writing into p is the
// fault, so the repair is the author's.
//
// The rule is adapted from staticcheck's SA1023, which reports a store to an
// element of p and an append to p.
package nilwrite

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/format"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilwrite",
	Doc:  "reports a Write method that may modify the buffer it was given",
	Run:  run,
}

var (
	byteSlice = types.NewSlice(types.Typ[types.Byte])
	errorType = types.Universe.Lookup("error").Type()
	writeSig  = newSignature(byteSlice, types.Typ[types.Int], errorType)
)

// newSignature returns the type of a method that takes one argument and
// returns results. It carries no receiver, because types.Identical ignores
// receivers when it compares two signatures.
func newSignature(arg types.Type, results ...types.Type) *types.Signature {
	var list []*types.Var
	for _, t := range results {
		list = append(list, types.NewVar(0, nil, "", t))
	}
	return types.NewSignatureType(nil, nil, nil,
		types.NewTuple(types.NewVar(0, nil, "", arg)),
		types.NewTuple(list...),
		false,
	)
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if decl, ok := n.(*ast.FuncDecl); ok {
				check(pass, decl)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each write into the parameter of decl's Write method.
func check(pass *analysis.Pass, decl *ast.FuncDecl) {
	if decl.Recv == nil || decl.Body == nil || decl.Name.Name != "Write" {
		return
	}
	obj, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
	if !ok {
		return
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || !types.Identical(sig, writeSig) {
		return
	}
	param := sig.Params().At(0)
	report := func(at ast.Node) {
		pass.Reportf(at.Pos(), "Write may modify the buffer it was given")
	}
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if element(pass, lhs, param) {
					report(lhs)
				}
			}
		case *ast.IncDecStmt:
			if element(pass, n.X, param) {
				report(n.X)
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				if element(pass, n.Key, param) {
					report(n.Key)
				}
				if element(pass, n.Value, param) {
					report(n.Value)
				}
			}
		case *ast.CallExpr:
			if writes(pass, n, param) {
				report(n)
			}
		}
		return true
	})
}

// writes reports whether call writes into the parameter through append, copy,
// or clear.
func writes(pass *analysis.Pass, call *ast.CallExpr, param *types.Var) bool {
	switch {
	case isBuiltin(pass, call, "append"):
		if len(call.Args) < 2 {
			return false
		}
		// A three-index slice with equal high and max bounds has no spare
		// capacity, so append allocates.
		slice, ok := ast.Unparen(call.Args[0]).(*ast.SliceExpr)
		if ok && slice.Slice3 && sameExpr(pass, slice.High, slice.Max) {
			return false
		}
		return into(pass, call.Args[0], param)
	case isBuiltin(pass, call, "copy"):
		return len(call.Args) > 1 && into(pass, call.Args[0], param) &&
			!emptySlice(pass, call.Args[0]) &&
			!emptySource(pass, call.Args[1])
	case isBuiltin(pass, call, "clear"):
		return len(call.Args) > 0 && into(pass, call.Args[0], param) &&
			!emptySlice(pass, call.Args[0])
	}
	return false
}

// sameExpr reports whether a and b are equal constants or the same
// side-effect-free expression.
func sameExpr(pass *analysis.Pass, a, b ast.Expr) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if at, ok := pass.TypesInfo.Types[a]; ok && at.Value != nil {
		if bt, ok := pass.TypesInfo.Types[b]; ok && bt.Value != nil {
			return constant.Compare(at.Value, token.EQL, bt.Value)
		}
	}
	if !sideEffectFree(pass, a) || !sideEffectFree(pass, b) {
		return false
	}
	var x, y bytes.Buffer
	if err := format.Node(&x, pass.Fset, a); err != nil {
		return false
	}
	if err := format.Node(&y, pass.Fset, b); err != nil {
		return false
	}
	return x.String() == y.String()
}

// sideEffectFree reports whether e has no call other than len or cap and no
// channel receive.
func sideEffectFree(pass *analysis.Pass, e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		if !ok {
			return false
		}
		switch n := n.(type) {
		case *ast.CallExpr:
			ok = isBuiltin(pass, n, "len") || isBuiltin(pass, n, "cap")
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				ok = false
			}
		}
		return ok
	})
	return ok
}

// emptySlice reports whether e is a slice expression whose length is the
// constant zero.
func emptySlice(pass *analysis.Pass, e ast.Expr) bool {
	slice, ok := ast.Unparen(e).(*ast.SliceExpr)
	if !ok || slice.High == nil {
		return false
	}
	var low int64
	if slice.Low != nil {
		low, ok = constantInt(pass, slice.Low)
		if !ok {
			return false
		}
	}
	high, ok := constantInt(pass, slice.High)
	return ok && low == high
}

// emptySource reports whether e is a typed nil slice, an empty string, an
// empty slice literal, or a slice expression with a constant zero length.
func emptySource(pass *analysis.Pass, e ast.Expr) bool {
	if emptySlice(pass, e) {
		return true
	}
	e = ast.Unparen(e)
	if lit, ok := e.(*ast.CompositeLit); ok && len(lit.Elts) == 0 {
		return true
	}
	if call, ok := e.(*ast.CallExpr); ok && len(call.Args) == 1 {
		tv, ok := pass.TypesInfo.Types[call.Fun]
		if ok && tv.IsType() {
			arg, ok := pass.TypesInfo.Types[call.Args[0]]
			if ok && arg.IsNil() {
				return true
			}
		}
	}
	tv, ok := pass.TypesInfo.Types[e]
	if !ok {
		return false
	}
	return tv.Value != nil && tv.Value.Kind() == constant.String &&
		constant.StringVal(tv.Value) == ""
}

// constantInt returns the value of e when e is a constant integer.
func constantInt(pass *analysis.Pass, e ast.Expr) (int64, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil {
		return 0, false
	}
	v, ok := constant.Int64Val(constant.ToInt(tv.Value))
	return v, ok
}

// isBuiltin reports whether call calls the builtin name. A package-level
// function of the same name is a different object.
func isBuiltin(pass *analysis.Pass, call *ast.CallExpr, name string) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup(name)
}

// element reports whether e names an element of the parameter or of a slice of
// it, as p[i] or p[1:][i].
func element(pass *analysis.Pass, e ast.Expr, param *types.Var) bool {
	index, ok := ast.Unparen(e).(*ast.IndexExpr)
	return ok && into(pass, index.X, param)
}

// into reports whether e names the parameter or a slice of it.
func into(pass *analysis.Pass, e ast.Expr, param *types.Var) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[e] == param
	case *ast.IndexExpr:
		return into(pass, e.X, param)
	case *ast.SliceExpr:
		return into(pass, e.X, param)
	}
	return false
}
