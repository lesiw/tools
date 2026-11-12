// Package unusedparams provides an analysis.Analyzer that reports a parameter
// a function never reads.
//
// This mirrors gopls' unusedparams
// (gopls/internal/analysis/unusedparams, golang.org/x/tools/gopls), which is
// internal and cannot be imported. The rule for a declared function is the
// same; gopls also reports a func literal, which this does not.
//
// A parameter that is never read is either a leftover or a signature the
// author meant to keep. Renaming it to the blank identifier says which, and
// costs nothing. The rule stands down where the name may be read or copied
// from elsewhere: an exported function or method is documented by its
// parameter names, an unexported method whose name matches an interface method
// in the package may be written to match it, and any function used as a value
// rather than called may have to match a func type. A function with no body,
// or one that only panics, is a placeholder, so its parameters stay as
// written.
//
// A func literal's parameters are left alone. The signature it is assigned to
// is written somewhere else, and the literal cannot show whether it was
// chosen.
package unusedparams

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedparams",
	Doc:  "reports a parameter a function never reads",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		info  = pass.TypesInfo
		calls = callIdents(pass)
		iface = interfaceMethods(pass)
		read  = readVars(pass)
		held  = heldFuncs(pass, calls)
	)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if fn.Name.IsExported() {
				continue // another package may take its address
			}
			if _, conforms := iface[fn.Name.Name]; fn.Recv != nil && conforms {
				continue // the signature may conform to the interface
			}
			if _, isHeld := held[info.Defs[fn.Name]]; isHeld {
				continue // used as a value, so the signature is fixed
			}
			if placeholder(fn.Body) {
				continue
			}
			report(pass, fn, read)
		}
	}
	return nil, nil
}

// report names each parameter of fn that nothing reads.
func report(pass *analysis.Pass, fn *ast.FuncDecl, read map[*types.Var]struct{}) {
	for _, field := range fn.Type.Params.List {
		for _, id := range field.Names {
			if id.Name == "_" {
				continue
			}
			param, ok := pass.TypesInfo.Defs[id].(*types.Var)
			if !ok {
				continue
			}
			if _, used := read[param]; used {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:     id.Pos(),
				End:     id.End(),
				Message: "parameter " + id.Name + " is never read",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Rename the parameter to _",
					TextEdits: []analysis.TextEdit{{
						Pos:     id.Pos(),
						End:     id.End(),
						NewText: []byte("_"),
					}},
				}},
			})
		}
	}
}

// callIdents returns the identifiers that name a called function, which are
// the uses that do not take the function's address.
func callIdents(pass *analysis.Pass) map[*ast.Ident]struct{} {
	out := make(map[*ast.Ident]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id := calledIdent(pass, call.Fun); id != nil {
				out[id] = struct{}{}
			}
			return true
		})
	}
	return out
}

// calledIdent returns the identifier that names the function a call calls. It
// looks through parentheses and a type instantiation, so (f)(1) and a call to
// f instantiated at a type both name f. An index that is not a type indexes a
// value, so the call goes through what the index names.
func calledIdent(pass *analysis.Pass, e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			if !pass.TypesInfo.Types[x.Index].IsType() {
				return nil
			}
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			return x.Sel
		default:
			return nil
		}
	}
}

// interfaceMethods returns the names of the unexported methods that any
// interface in the package declares.
func interfaceMethods(pass *analysis.Pass) map[string]struct{} {
	out := make(map[string]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			it, ok := n.(*ast.InterfaceType)
			if !ok {
				return true
			}
			for _, field := range it.Methods.List {
				for _, name := range field.Names {
					if !name.IsExported() && name.Name != "_" {
						out[name.Name] = struct{}{}
					}
				}
			}
			return true
		})
	}
	return out
}

// readVars returns the variables the package reads.
func readVars(pass *analysis.Pass) map[*types.Var]struct{} {
	out := make(map[*types.Var]struct{})
	for _, obj := range pass.TypesInfo.Uses {
		v, ok := obj.(*types.Var)
		if ok {
			out[v] = struct{}{}
		}
	}
	return out
}

// heldFuncs returns the functions the package uses as a value rather than
// calls.
func heldFuncs(pass *analysis.Pass, calls map[*ast.Ident]struct{}) map[types.Object]struct{} {
	out := make(map[types.Object]struct{})
	for id, obj := range pass.TypesInfo.Uses {
		if _, called := calls[id]; called {
			continue
		}
		if _, ok := obj.(*types.Func); ok && !obj.Exported() {
			out[obj] = struct{}{}
		}
	}
	return out
}

// placeholder reports whether body is empty or only panics, which marks a
// function written as a stub.
func placeholder(body *ast.BlockStmt) bool {
	switch len(body.List) {
	case 0:
		return true
	case 1:
		stmt, ok := body.List[0].(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		fun, ok := call.Fun.(*ast.Ident)
		return ok && fun.Name == "panic"
	}
	return false
}
