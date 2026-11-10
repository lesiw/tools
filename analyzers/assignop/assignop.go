// Package assignop provides an analysis.Analyzer that reports an assignment
// that writes a variable in terms of itself.
//
// x = x + y is x += y written out. The compound form names the variable once
// and the operation once. Like go-critic's assignOp.
package assignop

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "assignop",
	Doc:  "reports an assignment that writes a variable in terms of itself",
	Run:  run,
}

// compound maps a binary operator to the assignment that replaces it.
var compound = map[token.Token]token.Token{
	token.ADD:     token.ADD_ASSIGN,
	token.SUB:     token.SUB_ASSIGN,
	token.MUL:     token.MUL_ASSIGN,
	token.QUO:     token.QUO_ASSIGN,
	token.REM:     token.REM_ASSIGN,
	token.AND:     token.AND_ASSIGN,
	token.OR:      token.OR_ASSIGN,
	token.XOR:     token.XOR_ASSIGN,
	token.SHL:     token.SHL_ASSIGN,
	token.SHR:     token.SHR_ASSIGN,
	token.AND_NOT: token.AND_NOT_ASSIGN,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok || assign.Tok != token.ASSIGN {
				return true
			}
			if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				return true
			}
			lhs, ok := assign.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			bin, ok := assign.Rhs[0].(*ast.BinaryExpr)
			if !ok || compound[bin.Op] == token.ILLEGAL {
				return true
			}
			operand, ok := bin.X.(*ast.Ident)
			obj := pass.TypesInfo.ObjectOf(lhs)
			if !ok || pass.TypesInfo.ObjectOf(operand) != obj {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos: assign.Pos(),
				End: assign.End(),
				Message: "assignment writes " + lhs.Name +
					" in terms of itself; use " +
					compound[bin.Op].String(),
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Use the compound assignment",
					TextEdits: []analysis.TextEdit{{
						Pos:     assign.TokPos,
						End:     assign.TokPos + 1,
						NewText: []byte(compound[bin.Op].String()),
					}, {
						Pos: bin.X.Pos(),
						End: bin.Y.Pos(),
					}},
				}},
			})
			return true
		})
	}
	return nil, nil
}
