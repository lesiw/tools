// Package deferbeforeerr provides an analysis.Analyzer that reports a deferred
// call made before the error result is read.
//
// The spec's Defer statements section says the function value and the
// arguments are evaluated when the defer executes. So in
//
//	res, err := open()
//	defer res.Close()
//	if err != nil {
//		return err
//	}
//
// the receiver is taken where the defer stands, and the if statement that
// returns the error runs afterward, so the deferred call runs on a value that
// may not be usable.
//
// Go places the error last by convention, and the Google Go Style Guide states
// it: "By convention, error is the last result parameter." The last result is
// the one the rule reads to decide whether the others are usable.
//
// There is no fix: the deferred call moves below the error handling, and the
// tool cannot tell where the handling ends, or where to put the call when the
// error is discarded.
//
// The rule is adapted from staticcheck's SA5001. The README catalogs every
// check, delta, and exemption.
package deferbeforeerr

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "deferbeforeerr",
	Doc:  "reports a deferred call made before the error result is read",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	reported := make(map[token.Pos]bool) // one report per deferred call
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			var list []ast.Stmt
			switch n := n.(type) {
			case *ast.BlockStmt:
				list = n.List
			case *ast.CaseClause:
				list = n.Body
			case *ast.CommClause:
				list = n.Body
			default:
				return true
			}
			for i, stmt := range list {
				check(pass, reported, stmt, list[i+1:])
			}
			return true
		})
	}
	return nil, nil
}

// check reports a defer in rest that takes a value the error result decides,
// when nothing in rest reads the error first.
func check(pass *analysis.Pass, reported map[token.Pos]bool, stmt ast.Stmt, rest []ast.Stmt) {
	acq, ok := acquired(pass, stmt)
	if !ok {
		return
	}
	for _, later := range rest {
		if acq.err != nil && mentions(pass.TypesInfo, later, acq.err) {
			return
		}
		if binds(pass.TypesInfo, later, acq.res) {
			return
		}
		def, ok := later.(*ast.DeferStmt)
		if !ok || reported[def.Pos()] {
			continue
		}
		if !mentions(pass.TypesInfo, def.Call, acq.res...) {
			continue
		}
		reported[def.Pos()] = true
		pass.Reportf(def.Pos(), "%s", acq.message(def))
	}
}

// acquisition is a statement that binds the results of one call, with the
// values the error result decides and the error result, which is nil when the
// statement discards it.
type acquisition struct {
	call *ast.CallExpr
	res  []types.Object
	err  types.Object
}

// acquired returns what stmt binds from one call, and reports false for a
// statement that binds nothing.
func acquired(pass *analysis.Pass, stmt ast.Stmt) (acquisition, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) < 2 {
		return acquisition{}, false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !ok {
		return acquisition{}, false
	}
	sig, ok := pass.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok || sig.Results().Len() != len(assign.Lhs) {
		return acquisition{}, false
	}
	last := len(assign.Lhs) - 1
	errType := types.Universe.Lookup("error").Type()
	if !types.Identical(sig.Results().At(last).Type(), errType) {
		return acquisition{}, false
	}
	acq := acquisition{call: call}
	// A result written somewhere other than an identifier cannot be followed,
	// so the assignment is left alone rather than read as a discarded error.
	id, ok := ast.Unparen(assign.Lhs[last]).(*ast.Ident)
	if !ok {
		return acquisition{}, false
	}
	if id.Name != "_" {
		acq.err = pass.TypesInfo.ObjectOf(id)
		if acq.err == nil {
			return acquisition{}, false
		}
	}
	for _, lhs := range assign.Lhs[:last] {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
			acq.res = append(acq.res, obj)
		}
	}
	return acq, true
}

// message names the deferred call and the error it runs ahead of.
func (a acquisition) message(def *ast.DeferStmt) string {
	deferred := types.ExprString(def.Call)
	if a.err == nil {
		return fmt.Sprintf(
			"%s is deferred and the error from %s is discarded",
			deferred,
			types.ExprString(a.call),
		)
	}
	return fmt.Sprintf(
		"%s is deferred before %s is checked",
		deferred, a.err.Name(),
	)
}

// mentions reports whether n names one of objs. A function literal's body is
// not read: it names a value when it runs, not when the statement that holds
// the literal executes.
func mentions(info *types.Info, n ast.Node, objs ...types.Object) bool {
	var found bool
	ast.Inspect(n, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.Ident:
			found = slices.Contains(objs, info.ObjectOf(n))
		}
		return true
	})
	return found
}

// binds reports whether stmt assigns one of objs, which ends the scan: the
// error no longer decides the value bound there.
func binds(info *types.Info, stmt ast.Stmt, objs []types.Object) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok {
		return false
	}
	return slices.ContainsFunc(assign.Lhs, func(lhs ast.Expr) bool {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		return ok && slices.Contains(objs, info.ObjectOf(id))
	})
}
