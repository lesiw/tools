// Package marshalrecv provides an analysis.Analyzer that reports a marshal
// method that writes to a value receiver.
//
// A method declared with a value receiver works on a copy of the value it was
// called for, which Effective Go names in "Pointers vs. Values", so a write to
// the receiver lands in that copy and the caller never sees it.
//
// The write a method reads back is reported too, because the source does not
// say whether the write was meant to outlive the call.
//
// There is no suggested fix: the rewrite renames every use of the receiver in
// the body and needs a fresh name, which x/tools does not yet export a way to
// choose.
//
// The idea comes from revive's modifies-value-receiver, which reports the
// write in any method; this narrows it to the standard library's four marshal
// methods.
package marshalrecv

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "marshalrecv",
	Doc:  "reports a marshal method that writes to a value receiver",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !marshalMethod(pass, fn) {
				continue
			}
			recv := receiver(pass, fn)
			if recv == nil {
				continue
			}
			pos := firstWrite(pass, fn.Body, recv)
			if !pos.IsValid() {
				continue
			}
			name := types.TypeString(recv.Type(), types.RelativeTo(pass.Pkg))
			pass.Reportf(pos, "writes to a copy of %s; the caller never sees it", name)
		}
	}
	return nil, nil
}

// marshalMethod reports whether fn is one of the four marshal methods, which
// share the signature () ([]byte, error).
func marshalMethod(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	switch fn.Name.Name {
	case "MarshalJSON", "MarshalText", "MarshalBinary", "GobEncode":
	default:
		return false
	}
	sig, ok := pass.TypesInfo.Defs[fn.Name].Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 2 {
		return false
	}
	bytes := types.NewSlice(types.Typ[types.Uint8])
	err := types.Universe.Lookup("error").Type()
	return types.Identical(sig.Results().At(0).Type(), bytes) &&
		types.Identical(sig.Results().At(1).Type(), err)
}

// receiver returns the method's receiver variable, or nil for a pointer
// receiver, which already shares the caller's storage.
func receiver(pass *analysis.Pass, fn *ast.FuncDecl) *types.Var {
	if fn.Recv == nil || len(fn.Recv.List[0].Names) != 1 {
		return nil
	}
	recv, ok := pass.TypesInfo.Defs[fn.Recv.List[0].Names[0]].(*types.Var)
	if !ok {
		return nil
	}
	if _, ok := recv.Type().(*types.Pointer); ok {
		return nil
	}
	return recv
}

// firstWrite returns the position of the first write in body that lands in the
// copy, or token.NoPos. One report is enough to show the receiver is a copy.
func firstWrite(pass *analysis.Pass, body *ast.BlockStmt, recv *types.Var) token.Pos {
	pos := token.NoPos
	ast.Inspect(body, func(n ast.Node) bool {
		if pos.IsValid() {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if inCopy(pass, lhs, recv) {
					pos = lhs.Pos()
					break
				}
			}
		case *ast.IncDecStmt:
			if inCopy(pass, n.X, recv) {
				pos = n.X.Pos()
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				if inCopy(pass, n.Key, recv) {
					pos = n.Key.Pos()
				} else if n.Value != nil && inCopy(pass, n.Value, recv) {
					pos = n.Value.Pos()
				}
			}
		}
		return true
	})
	return pos
}

// inCopy reports whether e names storage inside the receiver's copy: the
// receiver, a field of it, or an element of an array it holds, reached
// directly or through the receiver's address. A field reached through a
// pointer, and an element of a slice or a map, lead to storage the caller
// shares, so a write there is not lost.
func inCopy(pass *analysis.Pass, e ast.Expr, recv *types.Var) bool {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return false
			}
			e = x.X
		case *ast.Ident:
			return pass.TypesInfo.ObjectOf(x) == recv
		case *ast.SelectorExpr:
			sel, ok := pass.TypesInfo.Selections[x]
			if !ok {
				return false
			}
			if sel.Indirect() {
				and, ok := ast.Unparen(x.X).(*ast.UnaryExpr)
				if !ok || and.Op != token.AND {
					return false
				}
			}
			e = x.X
		case *ast.IndexExpr:
			if !arrayed(pass, x.X) {
				return false
			}
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		default:
			return false
		}
	}
}

// arrayed reports whether e is an array, or a slice over one.
func arrayed(pass *analysis.Pass, e ast.Expr) bool {
	if t := pass.TypesInfo.TypeOf(e); t != nil {
		if _, ok := t.Underlying().(*types.Array); ok {
			return true
		}
	}
	s, ok := e.(*ast.SliceExpr)
	return ok && arrayed(pass, s.X)
}
