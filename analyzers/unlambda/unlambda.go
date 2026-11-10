// Package unlambda provides an analysis.Analyzer that reports a function
// literal that only forwards its arguments to another function.
//
// A literal whose body is one call, with its own parameters in order, adds
// nothing to the call. Passing the function value is shorter, and it puts the
// name of the function at the call site.
package unlambda

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unlambda",
	Doc:  "reports a function literal that only forwards its arguments",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.FuncLit)
			if !ok || !forwards(pass, lit) {
				return true
			}
			ret := lit.Body.List[0].(*ast.ReturnStmt)
			call := ret.Results[0].(*ast.CallExpr)
			pass.Report(analysis.Diagnostic{
				Pos:     lit.Pos(),
				End:     lit.End(),
				Message: "literal only forwards its arguments; pass the function instead",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Pass the function",
					TextEdits: []analysis.TextEdit{{
						Pos:     lit.Pos(),
						End:     lit.End(),
						NewText: []byte(types.ExprString(call.Fun)),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}

// forwards reports whether lit is one call whose arguments are its own
// parameters, in order.
func forwards(pass *analysis.Pass, lit *ast.FuncLit) bool {
	if len(lit.Body.List) != 1 {
		return false
	}
	ret, ok := lit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	if !ok || !callable(pass, call.Fun) {
		return false
	}
	params := names(lit.Type.Params)
	if len(params) != len(call.Args) {
		return false
	}
	for i, arg := range call.Args {
		id, ok := arg.(*ast.Ident)
		if !ok || id.Name != params[i] {
			return false
		}
	}
	return true
}

// callable reports whether e names a function or a package's function. A
// method value closes over its receiver, so forwarding to one is not a
// forwarding literal.
func callable(pass *analysis.Pass, e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.IndexExpr:
		return callable(pass, e.X)
	case *ast.IndexListExpr:
		return callable(pass, e.X)
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		ident, ok := e.X.(*ast.Ident)
		if !ok {
			return false
		}
		_, ok = pass.TypesInfo.Uses[ident].(*types.PkgName)
		return ok
	}
	return false
}

// names returns the parameter names of a field list.
func names(fields *ast.FieldList) (out []string) {
	if fields == nil {
		return nil
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			out = append(out, name.Name)
		}
	}
	return out
}
