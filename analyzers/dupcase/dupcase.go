// Package dupcase provides an analysis.Analyzer that reports a case that
// names an expression an earlier case already named.
//
// The second case can never match, so the author either mistyped one of them
// or meant to fold them together. Constants are caught by the compiler;
// expressions that are not constant are not.
package dupcase

import (
	"bytes"
	"go/ast"
	"go/printer"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupcase",
	Doc:  "reports a case that names an expression an earlier case named",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch stmt := n.(type) {
			case *ast.SwitchStmt:
				checkCases(pass, stmt.Body)
			case *ast.TypeSwitchStmt:
				// A repeated type in a type switch does not compile.
			}
			return true
		})
	}
	return nil, nil
}

// checkCases reports each case expression that repeats an earlier one.
func checkCases(pass *analysis.Pass, body *ast.BlockStmt) {
	seen := make(map[string]struct{})
	for _, stmt := range body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, expr := range clause.List {
			key := text(pass, expr)
			if key == "" || pass.TypesInfo.Types[expr].Value != nil {
				continue // a repeated constant does not compile
			}
			if _, ok := seen[key]; ok {
				pass.Reportf(expr.Pos(), "case repeats an earlier expression; it can never match")
				continue
			}
			seen[key] = struct{}{}
		}
	}
}

// text returns the printed expression, which is what makes two of them equal.
func text(pass *analysis.Pass, e ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, pass.Fset, e); err != nil {
		return ""
	}
	return buf.String()
}
