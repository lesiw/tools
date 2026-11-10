// Package unusedrecv provides an analysis.Analyzer that reports a receiver
// whose name the method body never reads.
//
// Go allows a receiver with no name. A name the body never reads suggests the
// method works on the value, and a reader has to check the body to learn that
// it does not.
package unusedrecv

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedrecv",
	Doc:  "reports a receiver that the method body never reads",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			// A declaration without a body reads nothing, and it is not
			// the method's business to say so.
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			for _, field := range fn.Recv.List {
				check(pass, fn, field)
			}
		}
	}
	return nil, nil
}

// check reports each name in field that fn's body never reads.
func check(pass *analysis.Pass, fn *ast.FuncDecl, field *ast.Field) {
	for _, id := range field.Names {
		if id.Name == "_" {
			continue // the blank name is already the statement
		}
		obj := pass.TypesInfo.Defs[id]
		if obj == nil || reads(pass, fn.Body, obj) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     id.Pos(),
			End:     id.End(),
			Message: "receiver " + id.Name + " is unused",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: "Drop the name",
				TextEdits: []analysis.TextEdit{{
					Pos: id.Pos(),
					End: field.Type.Pos(),
				}},
			}},
		})
	}
}

// reads reports whether body reads obj, including from a nested literal.
func reads(pass *analysis.Pass, body *ast.BlockStmt, obj types.Object) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok || pass.TypesInfo.ObjectOf(id) != obj {
			return true
		}
		// The defining identifier is not a read of the name.
		found = pass.TypesInfo.Uses[id] == obj
		return true
	})
	return found
}
