// Package switchtrue provides an analysis.Analyzer that reports a switch
// whose tag is the literal true.
//
// A switch with no tag compares every case with true. Writing true as the tag
// gives the switch one word it does not need.
package switchtrue

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "switchtrue",
	Doc:  "reports a switch whose tag is the literal true",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.SwitchStmt)
			if !ok || !isTrue(pass, stmt.Tag) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     stmt.Switch,
				End:     stmt.Tag.End(),
				Message: "switch on true; drop the tag",
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   "Drop the tag",
					TextEdits: dropTag(stmt),
				}},
			})
			return true
		})
	}
	return nil, nil
}

// dropTag returns the edit that removes the tag, keeping an init statement.
func dropTag(stmt *ast.SwitchStmt) []analysis.TextEdit {
	if stmt.Init != nil {
		// switch m := n; { keeps the statement and drops the tag.
		return []analysis.TextEdit{{
			Pos: stmt.Tag.Pos(),
			End: stmt.Tag.End(),
		}}
	}
	return []analysis.TextEdit{{
		Pos:     stmt.Switch,
		End:     stmt.Tag.End(),
		NewText: []byte("switch"),
	}}
}

// isTrue reports whether e is the predeclared true.
func isTrue(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("true")
}
