// Package earlyreturn provides an analysis.Analyzer that reports a return that
// chains conditions past the width.
//
// A chain that mixes && and || makes the reader work out the precedence as
// well as the conditions, and a second line makes that worse. Early returns
// decompose it: a guard for each way out, and the answer at the end. The rule
// reports the shape and points at the remedy. It does not rewrite, because
// decomposition changes control flow and the fix would need to name each part.
//
// A return whose value is not a chain of conditions is left alone however it
// wraps. The fault is a return of a condition that cannot hold one line, where
// one test has become several.
//
// A chain that uses one operator kind is left alone however long it is. A
// conjunction of three has no precedence to work out.
//
// A header that wraps has the same problem, and linewrap reports it.
package earlyreturn

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "earlyreturn",
	Doc:  "reports a return that chains conditions past the width",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if linelen.Config.Len <= 0 {
		return nil, nil // skip the render; CheckLine allows every line
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return true
			}
			expr := ret.Results[0]
			bin, ok := ast.Unparen(expr).(*ast.BinaryExpr)
			if !ok || !logical(bin.Op) {
				return true
			}
			if !has(bin, token.LAND) || !has(bin, token.LOR) {
				return true
			}
			count := chain(bin)
			line, ok := joined(pass, ret, expr)
			if !ok || linelen.CheckLine(line) {
				return true // the join rule fits it on one line
			}
			pass.Reportf(
				ret.Pos(),
				"return chains %d conditions; early returns decompose it",
				count,
			)
			return true
		})
	}
	return nil, nil
}

// logical reports whether op joins two conditions.
func logical(op token.Token) bool {
	return op == token.LAND || op == token.LOR
}

// chain returns the number of operands in the expression's tree of conditions.
func chain(e ast.Expr) (count int) {
	bin, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || !logical(bin.Op) {
		return 1
	}
	return chain(bin.X) + chain(bin.Y)
}

// has reports whether the tree of conditions under e holds op.
func has(e ast.Expr, op token.Token) bool {
	bin, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || !logical(bin.Op) {
		return false
	}
	return bin.Op == op || has(bin.X, op) || has(bin.Y, op)
}

// joined returns the line the return would print if the expression were
// written on one line.
func joined(pass *analysis.Pass, ret *ast.ReturnStmt, e ast.Expr) ([]byte, bool) {
	var buf bytes.Buffer
	if err := format.Node(&buf, pass.Fset, e); err != nil {
		return nil, false
	}
	tokFile := pass.Fset.File(ret.Pos())
	off := tokFile.Offset(ret.Pos())
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return nil, false
	}
	// The indent, measured the way the width rule measures it.
	var start int
	for start = off; start > 0 && src[start-1] != '\n'; start-- {
	}
	indent := linelen.Width(src[start:off])
	line := append(bytes.Repeat([]byte(" "), indent), "return "...)
	return append(line, buf.Bytes()...), true
}
