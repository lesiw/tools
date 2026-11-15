// Package initclause provides an analysis.Analyzer that reports an if or
// switch init clause holding a statement that declares nothing.
//
// The spec admits any simple statement in the clause. Effective Go describes
// the common one: "Since if and switch accept an initialization statement,
// it's common to see one used to set up a local variable." A call, a send, or
// an increment sets up no variable, so the clause reads as a declaration and
// is something else.
//
// A short variable declaration parses as an AssignStmt with token.DEFINE, so
// one type assertion exempts declarations and assignments alike.
//
// Report-only: hoisting a statement out of the clause loses the comment above
// it, an else if clause has no statement position to hoist into, and a
// labelled switch stops compiling once a statement is inserted under the
// label, because the label takes the hoisted statement.
//
// Like go-critic's initClause.
package initclause

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "initclause",
	Doc:  "reports an if or switch init clause that declares nothing",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			var (
				init ast.Stmt
				kind string
			)
			switch stmt := n.(type) {
			case *ast.IfStmt:
				init, kind = stmt.Init, "if"
			case *ast.SwitchStmt:
				init, kind = stmt.Init, "switch"
			case *ast.TypeSwitchStmt:
				init, kind = stmt.Init, "type switch"
			default:
				return true
			}
			if init == nil {
				return true
			}
			if _, ok := init.(*ast.AssignStmt); ok {
				return true
			}
			pass.Reportf(init.Pos(), "%s init clause declares nothing", kind)
			return true
		})
	}
	return nil, nil
}
