// Package increment provides an analysis.Analyzer that reports an increment or
// decrement written as a compound assignment.
//
// The spec gives x++ and x += 1 the same meaning. Go code mostly writes x++,
// and this rule reports the exceptions, rewriting x += 1 as x++. Only an
// operand whose underlying type is an integer is reported. The rule is adapted
// from revive's increment-decrement, which reports every operand type.
package increment

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "increment",
	Doc:  "reports an increment or decrement written as a compound assignment",
	Run:  run,
}

// operator maps a compound increment or decrement to the operator it becomes.
var operator = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.INC,
	token.SUB_ASSIGN: token.DEC,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			op, ok := operator[assign.Tok]
			if !ok {
				return true
			}
			if !isOne(assign.Rhs[0]) {
				return true
			}
			if !isInteger(pass.TypesInfo.TypeOf(assign.Lhs[0])) {
				return true
			}
			written, replacement := assign.Tok.String(), op.String()
			pass.Report(analysis.Diagnostic{
				Pos:     assign.Pos(),
				End:     assign.End(),
				Message: written + " 1 can be replaced by " + replacement,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message:   "Replace " + written + " 1 with " + replacement,
					TextEdits: edits(file, assign, written, replacement),
				}},
			})
			return true
		})
	}
	return nil, nil
}

// edits rewrites the operator and drops the literal. One edit spans both, so
// nothing is left between them; a comment in the span survives only when the
// two are rewritten separately. A dereference is parenthesized for the reader:
// *p++ already means (*p)++, but it reads as *(p++).
func edits(file *ast.File, assign *ast.AssignStmt, written, replacement string) []analysis.TextEdit {
	var (
		lo, hi = assign.Lhs[0].End(), assign.Rhs[0].End()
		out    = []analysis.TextEdit{{
			Pos:     lo,
			End:     hi,
			NewText: []byte(replacement),
		}}
	)
	if hasComment(file, lo, hi) {
		out = []analysis.TextEdit{{
			Pos:     assign.TokPos,
			End:     assign.TokPos + token.Pos(len(written)),
			NewText: []byte(replacement),
		}, {
			Pos: assign.Rhs[0].Pos(),
			End: hi,
		}}
	}
	if operand := assign.Lhs[0]; isStar(operand) {
		out = append(out, analysis.TextEdit{
			Pos:     operand.Pos(),
			End:     operand.Pos(),
			NewText: []byte("("),
		}, analysis.TextEdit{
			Pos:     operand.End(),
			End:     operand.End(),
			NewText: []byte(")"),
		})
	}
	return out
}

// hasComment reports whether a comment lies in the range [lo, hi).
func hasComment(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() >= lo && comment.End() <= hi {
				return true
			}
		}
	}
	return false
}

// isStar reports whether e is a dereference.
func isStar(e ast.Expr) bool {
	_, ok := e.(*ast.StarExpr)
	return ok
}

// isOne reports whether e is the literal 1.
func isOne(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "1"
}

// isInteger reports whether t's underlying type is an integer.
func isInteger(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}
