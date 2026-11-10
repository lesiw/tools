// Package nilreturn provides an analysis.Analyzer that reports a return of
// the very variable an if just proved to be nil.
//
// The if asked whether err is nil, and its body returns err, so the return
// always yields nil. Either the condition is inverted, or the return should
// name nil and say so.
package nilreturn

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilreturn",
	Doc:  "reports a return of the variable an if just found to be nil",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			obj, name, ok := nilCheck(pass, ifStmt.Cond)
			if !ok {
				return true
			}
			check(pass, ifStmt.Body, obj, name)
			return true
		})
	}
	return nil, nil
}

// check reports a return in body that returns the tested object as its only
// result. A tuple that includes the variable returns a value, not
// this one.
func check(pass *analysis.Pass, body *ast.BlockStmt, obj types.Object, name string) {
	// A nil-receiver guard returns the receiver, which is not an error and
	// is the usual way to say there is nothing to do.
	if !implementsError(obj.Type()) {
		return
	}
	// An assignment between the test and the return gives the variable a
	// value that is not nil.
	if reassigned(pass, body, obj) {
		return
	}
	ast.Inspect(body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return true
		}
		id, ok := ret.Results[0].(*ast.Ident)
		if !ok || pass.TypesInfo.ObjectOf(id) != obj {
			return true
		}
		pass.Reportf(id.Pos(), "%s is nil here, so the return is nil", name)
		return true
	})
}

// implementsError reports whether t implements the error interface.
func implementsError(t types.Type) bool {
	if t == nil {
		return false
	}
	var (
		errType = types.Universe.Lookup("error").Type()
		err, ok = errType.Underlying().(*types.Interface)
	)
	return ok && types.Implements(t, err)
}

// reassigned reports whether body assigns to obj.
func reassigned(pass *analysis.Pass, body *ast.BlockStmt, obj types.Object) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(id) == obj {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// nilCheck returns the object and name an equality with nil tests.
func nilCheck(pass *analysis.Pass, cond ast.Expr) (types.Object, string, bool) {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return nil, "", false
	}
	if id, ok := bin.X.(*ast.Ident); ok && isNil(pass, bin.Y) {
		return pass.TypesInfo.ObjectOf(id), id.Name, true
	}
	if id, ok := bin.Y.(*ast.Ident); ok && isNil(pass, bin.X) {
		return pass.TypesInfo.ObjectOf(id), id.Name, true
	}
	return nil, "", false
}

// isNil reports whether e is the predeclared nil.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("nil")
}
