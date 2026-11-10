// Package dupbranch provides an analysis.Analyzer that reports an if whose
// two branches contain the same statements.
//
// The condition decides nothing. Either it is left over from an edit, or one
// branch was meant to do something else.
package dupbranch

import (
	"bytes"
	"go/ast"
	"go/printer"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupbranch",
	Doc:  "reports an if whose two branches hold the same statements",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			elseBlock, ok := ifStmt.Else.(*ast.BlockStmt)
			if !ok || len(ifStmt.Body.List) == 0 {
				return true
			}
			if text(pass, ifStmt.Body.List) != text(pass, elseBlock.List) {
				return true
			}
			pass.Reportf(ifStmt.Else.Pos(), "both branches hold the same statements")
			return true
		})
	}
	return nil, nil
}

// text returns the printed statements of a block.
func text(pass *analysis.Pass, stmts []ast.Stmt) string {
	var buf bytes.Buffer
	for _, stmt := range stmts {
		if err := printer.Fprint(&buf, pass.Fset, stmt); err != nil {
			return ""
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
