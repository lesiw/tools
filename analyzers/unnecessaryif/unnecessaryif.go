// Package unnecessaryif provides an analysis.Analyzer that reports an if
// whose else block contains a single if.
//
// The IfStmt production of the Go spec admits an IfStmt after else, so the
// else { if ... } form and the else if ... form are the same statement. The
// second removes a pair of braces and one level of indent. The fix is withheld
// when it would delete a comment.
//
// The rule is adapted from go-critic's elseif. The README catalogs every
// check, exemption, and fix.
package unnecessaryif

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unnecessaryif",
	Doc:  "reports an if whose else block contains a single if",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			block, ok := stmt.Else.(*ast.BlockStmt)
			if !ok || len(block.List) != 1 {
				return true
			}
			nested, ok := block.List[0].(*ast.IfStmt)
			if !ok {
				return true
			}
			diagnostic := analysis.Diagnostic{
				Pos:     stmt.Else.Pos(),
				End:     block.End(),
				Message: "if can be written as else if",
			}
			if fixable(file, block, nested) {
				diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Write else if",
					TextEdits: []analysis.TextEdit{{
						Pos:     block.Lbrace,
						End:     nested.Pos(),
						NewText: []byte(" "),
					}, {
						Pos: nested.End(),
						End: block.Rbrace + 1,
					}},
				}}
			}
			pass.Report(diagnostic)
			return true
		})
	}
	return nil, nil
}

// fixable reports whether the edits that write else if delete no comment.
func fixable(file *ast.File, block *ast.BlockStmt, nested *ast.IfStmt) bool {
	for _, group := range file.Comments {
		if overlaps(group, block.Lbrace, nested.Pos()) {
			return false
		}
		if overlaps(group, nested.End(), block.Rbrace+1) {
			return false
		}
	}
	return true
}

// overlaps reports whether group has any part in the half-open range lo to hi.
func overlaps(group *ast.CommentGroup, lo, hi token.Pos) bool {
	return group.Pos() < hi && group.End() > lo
}
