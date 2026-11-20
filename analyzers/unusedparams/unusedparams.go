// Package unusedparams provides an analysis.Analyzer that reports a parameter
// a declared function never uses.
//
// Vendored from gopls' unusedparams
// (gopls/internal/analysis/unusedparams, golang.org/x/tools/gopls v0.24.0),
// which is internal and cannot be imported. The rule is upstream's; the code
// is adapted to this repository's helpers.
//
// Renaming a parameter to the blank identifier says the position is
// deliberate, and every report carries that fix. The README lists the shapes
// the rule leaves alone and the places it differs from upstream.
package unusedparams

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedparams",
	Doc:  "reports a parameter a declared function never uses",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		info    = pass.TypesInfo
		methods = uses.InterfaceMethods(pass)
		read    = readVars(pass)
		held    = uses.Held(pass)
	)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if fn.Name.IsExported() {
				continue // the names document the signature
			}
			self, ok := info.Defs[fn.Name].(*types.Func)
			if !ok {
				continue
			}
			if fn.Recv != nil {
				// As upstream has it, a name match is enough: a signature
				// that mentions a type parameter never compares identical.
				if _, ok := methods[fn.Name.Name]; ok {
					continue
				}
			}
			if _, isHeld := held[self]; isHeld {
				continue // the signature was chosen elsewhere
			}
			if placeholder(fn.Body) {
				continue
			}
			report(pass, fn, read)
		}
	}
	return nil, nil
}

// report names each parameter of fn that nothing uses.
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
				Message: "parameter " + id.Name + " is never used",
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

// readVars returns the variables the package refers to. A field is never a
// parameter, so it is not gathered.
func readVars(pass *analysis.Pass) map[*types.Var]struct{} {
	out := make(map[*types.Var]struct{})
	for _, obj := range pass.TypesInfo.Uses {
		v, ok := obj.(*types.Var)
		if ok && !v.IsField() {
			out[v] = struct{}{}
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
