// Package emptybranch provides an analysis.Analyzer that reports an if or else
// branch with no statements and no comment.
//
// The Go spec says a block is "a possibly empty sequence of declarations and
// statements and nested labeled statements"
// (https://go.dev/ref/spec#Blocks), so the compiler accepts an empty branch.
// An empty branch is often a placeholder for code not yet written, or a
// leftover from an edit.
//
// A comment inside the branch suppresses the report: it may explain why the
// branch is empty. The README catalogs every check and exemption.
//
// Report only: either branch may be a placeholder for code not yet written, so
// no rewrite is offered.
//
// The rule is adapted from staticcheck's SA9003. SA9003 reports empty
// branches, including one that holds only a comment; this rule leaves the
// commented branch alone. SA9003 also skips an empty then branch when the else
// has a body or is an else-if; this rule reports it.
package emptybranch

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "emptybranch",
	Doc:  "reports an if or else branch with no statements and no comment",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			report(pass, file, stmt.Body, "if")
			if block, ok := stmt.Else.(*ast.BlockStmt); ok {
				report(pass, file, block, "else")
			}
			return true
		})
	}
	return nil, nil
}

// report reports block as an empty branch of the kind named, unless it holds a
// statement or a comment.
func report(pass *analysis.Pass, file *ast.File, block *ast.BlockStmt, kind string) {
	if len(block.List) != 0 || commented(file, block) {
		return
	}
	pass.Reportf(block.Pos(), "empty %s branch", kind)
}

// commented reports whether a comment sits between the braces of block.
func commented(file *ast.File, block *ast.BlockStmt) bool {
	for _, group := range file.Comments {
		if group.Pos() < block.Rbrace && group.End() > block.Lbrace {
			return true
		}
	}
	return false
}
