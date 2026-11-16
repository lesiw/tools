// Package unreachablereturn provides an analysis.Analyzer that reports a
// return statement after a panic or an endless loop in the same block.
//
// An endless loop is a for statement that cannot complete normally.
//
// The spec treats a call to panic as terminating, and a for statement with no
// condition, no break referring to it, and no range clause as terminating
// (https://go.dev/ref/spec#Terminating_statements). Neither falls through, so
// a return after one cannot run unless a goto reaches it.
//
// A for statement with no condition is endless to the rule unless a break
// targets the loop. A break to an enclosing statement skips the rest of the
// block after the inner loop, so a return there is reported.
//
// A goto target is treated as reachable, even when the goto itself cannot run.
// A return after a loop that cannot complete normally, before the label of a
// forward goto out of the loop, is reported unless another goto target comes
// between them. A return after the label is reported when a panic or an
// endless loop comes between the label and the return.
//
// The rule checks block statements and the bodies of case and comm clauses.
//
// Report only: removing the return would drop any comment on it.
//
// The rule is inspired by the x/tools unreachable pass.
package unreachablereturn

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "unreachablereturn",
	Doc:  "reports a return statement after a panic or an endless loop in the same block",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				checkFunc(pass, file, n.Body)
				return false
			case *ast.FuncLit:
				checkFunc(pass, file, n.Body)
				return false
			}
			return true
		})
	}
	return nil, nil
}

// checkFunc reports the unreachable returns in body and the literals it
// contains.
func checkFunc(pass *analysis.Pass, file *ast.File, body *ast.BlockStmt) {
	if body == nil {
		return
	}
	labels, gotos := collect(body)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			checkFunc(pass, file, n.Body)
			return false
		case *ast.BlockStmt:
			checkList(pass, file, n.List, labels, gotos)
		case *ast.CaseClause:
			checkList(pass, file, n.Body, labels, gotos)
		case *ast.CommClause:
			checkList(pass, file, n.Body, labels, gotos)
		}
		return true
	})
}

// collect returns the statement under every label in the function and the set
// of labels that a goto names.
func collect(body *ast.BlockStmt) (map[string]ast.Stmt, map[string]bool) {
	var (
		labels = make(map[string]ast.Stmt)
		gotos  = make(map[string]bool)
	)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n.Stmt
		case *ast.BranchStmt:
			if n.Tok == token.GOTO && n.Label != nil {
				gotos[n.Label.Name] = true
			}
		}
		return true
	})
	return labels, gotos
}

// checkList reports the returns in list that follow a panic or a loop that
// cannot complete normally in the same list.
func checkList(pass *analysis.Pass, file *ast.File, list []ast.Stmt, labels map[string]ast.Stmt, gotos map[string]bool) {
	var (
		dead  bool
		cause string
	)
	for _, stmt := range list {
		// A goto target is treated as reachable, even when the goto itself
		// cannot run.
		if isGotoTarget(stmt, gotos) {
			dead, cause = false, ""
		}
		inner := unlabel(stmt)
		if dead {
			if ret, ok := inner.(*ast.ReturnStmt); ok {
				pass.Reportf(
					ret.Pos(),
					"return is unreachable after %s", cause,
				)
			}
		}
		switch {
		case isPanic(pass, inner):
			dead, cause = true, "a panic"
		case isEndless(file, inner, labels):
			dead, cause = true, "a loop that cannot complete normally"
		}
	}
}

// isGotoTarget reports whether stmt is a label that a goto names.
func isGotoTarget(stmt ast.Stmt, gotos map[string]bool) bool {
	labeled, ok := stmt.(*ast.LabeledStmt)
	return ok && gotos[labeled.Label.Name]
}

// unlabel returns the statement under any labels in front of stmt.
func unlabel(stmt ast.Stmt) ast.Stmt {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = labeled.Stmt
	}
}

// isPanic reports whether stmt is a call to the builtin panic.
func isPanic(pass *analysis.Pass, stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := ast.Unparen(expr.X).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	return pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic")
}

// isEndless reports whether stmt is a for statement with no condition and no
// break that leaves it for the following statement.
func isEndless(file *ast.File, stmt ast.Stmt, labels map[string]ast.Stmt) bool {
	loop, ok := stmt.(*ast.ForStmt)
	if !ok || loop.Cond != nil {
		return false
	}
	var exits bool
	ast.Inspect(loop.Body, func(n ast.Node) bool {
		if exits {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if n.Tok == token.BREAK && breaksLoop(file, loop, n, labels) {
				exits = true
			}
		}
		return true
	})
	return !exits
}

// breaksLoop reports whether branch leaves loop for the code after it.
func breaksLoop(file *ast.File, loop *ast.ForStmt, branch *ast.BranchStmt, labels map[string]ast.Stmt) bool {
	if branch.Label != nil {
		return labels[branch.Label.Name] == ast.Stmt(loop)
	}
	path, _ := astutil.PathEnclosingInterval(file, branch.Pos(), branch.End())
	for _, node := range path {
		if node == ast.Node(loop) {
			return true
		}
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		}
	}
	return false
}
