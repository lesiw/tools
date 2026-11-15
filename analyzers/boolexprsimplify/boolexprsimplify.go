// Package boolexprsimplify provides an analysis.Analyzer that reports a
// boolean expression with a redundant negation or comparison.
//
// A logical operator yields a result of the same type as its operands, so !!x
// is x. A comparison with a boolean constant yields an untyped boolean value,
// which any boolean type takes, so x == true has the value of x. A name means
// only what its type allows: where a named boolean type is wanted, the untyped
// value fits and a bool does not.
//
// The type checker records the type the comparison's value was given, so the
// rule reads that record. A few shapes are reported without a fix, and the
// README catalogs every check and exemption.
//
// The rule is like go-critic's boolExprSimplify, which also inverts a negated
// comparison, and like staticcheck's S1002, which compares with a boolean
// constant.
package boolexprsimplify

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "boolexprsimplify",
	Doc:  "reports a boolean expression with a redundant negation or comparison",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.UnaryExpr:
				// A chain of four waits for the next run, because a report
				// covers one pair, and a comparison inside the operand is
				// left for that run too.
				if negation(pass, file, n) {
					return false
				}
			case *ast.BinaryExpr:
				comparison(pass, file, n)
			}
			return true
		})
	}
	return nil, nil
}

// negation reports a double negation and whether it found one.
func negation(pass *analysis.Pass, file *ast.File, outer *ast.UnaryExpr) bool {
	if outer.Op != token.NOT {
		return false
	}
	inner, ok := ast.Unparen(outer.X).(*ast.UnaryExpr)
	if !ok || inner.Op != token.NOT {
		return false
	}
	text := astexpr.Text(inner.X)
	d := analysis.Diagnostic{
		Pos:     outer.Pos(),
		End:     outer.End(),
		Message: "negation of a negation; write the operand",
	}
	// The operand replaces the expression on one line, so its own parentheses
	// travel with it. A comment inside the expression would be lost, a
	// statement header cannot begin with a composite literal, which the
	// operand's parentheses are not there to prevent, and the line the
	// operand lands on is the author's to fit.
	fixable := !composite(inner.X) && fits(pass, outer, text) &&
		!commented(file.Comments, outer.Pos(), outer.End())
	if fixable {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Write the operand",
			TextEdits: []analysis.TextEdit{{
				Pos:     outer.Pos(),
				End:     outer.End(),
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(d)
	return true
}

// composite reports whether e begins with a composite literal.
func composite(e ast.Expr) bool {
	for {
		switch n := e.(type) {
		case *ast.SelectorExpr:
			e = n.X
		case *ast.IndexExpr:
			e = n.X
		case *ast.IndexListExpr:
			e = n.X
		case *ast.CallExpr:
			e = n.Fun
		case *ast.SliceExpr:
			e = n.X
		case *ast.TypeAssertExpr:
			e = n.X
		case *ast.CompositeLit:
			return true
		default:
			return false
		}
	}
}

// comparison reports a comparison that restates its operand.
func comparison(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) {
	operand := restated(pass, bin)
	if operand == nil {
		return
	}
	// The checker gave the comparison the type the context wants, or left it
	// untyped where nothing takes the value.
	if !isBool(pass.TypesInfo.TypeOf(bin)) {
		return
	}
	// The deleted span runs from the operand's end to the expression's end,
	// unless the constant comes first.
	pos, end := operand.End(), bin.End()
	if operand == bin.Y {
		pos, end = bin.Pos(), operand.Pos()
	}
	message := "comparison with false; write the operand"
	if bin.Op == token.EQL {
		message = "comparison with true; write the operand"
	}
	d := analysis.Diagnostic{Pos: bin.Pos(), End: bin.End(), Message: message}
	if !commented(file.Comments, pos, end) {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Drop the comparison",
			TextEdits: []analysis.TextEdit{{Pos: pos, End: end}},
		}}
	}
	pass.Report(d)
}

// restated returns the expression a comparison restates, or nil when the
// comparison says more than that.
func restated(pass *analysis.Pass, bin *ast.BinaryExpr) (operand ast.Expr) {
	if bin.Op != token.EQL && bin.Op != token.NEQ {
		return nil
	}
	// == agrees with true and != with false. The constant that disagrees
	// inverts the operand, and the rewrite would add a negation.
	agree := bin.Op == token.EQL
	if value, ok := truth(pass, bin.Y); ok && value == agree {
		operand = bin.X
	} else if value, ok := truth(pass, bin.X); ok && value == agree {
		operand = bin.Y
	}
	if operand == nil {
		return nil
	}
	// Two constants make a constant expression, which has no operand.
	if _, ok := truth(pass, operand); ok {
		return nil
	}
	// A named boolean type is a different type, and the value in its place
	// would carry that type instead of the comparison's.
	if !isBool(pass.TypesInfo.TypeOf(operand)) {
		return nil
	}
	return operand
}

// truth returns the value of the predeclared boolean constant e names, and
// whether e names one.
func truth(pass *analysis.Pass, e ast.Expr) (value, ok bool) {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false, false
	}
	switch pass.TypesInfo.Uses[id] {
	case types.Universe.Lookup("true"):
		return true, true
	case types.Universe.Lookup("false"):
		return false, true
	}
	return false, false
}

// isBool reports whether t is bool or the type of an untyped boolean value.
func isBool(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

// fits reports whether text keeps the line the expression sits on within the
// width limit.
func fits(pass *analysis.Pass, outer *ast.UnaryExpr, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	file := pass.Fset.File(outer.Pos())
	if file == nil {
		return false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	start := file.Offset(outer.Pos())
	end := file.Offset(outer.End())
	begin := bytes.LastIndexByte(src[:start], '\n') + 1
	line := append(slices.Clone(src[begin:start]), text...)
	if tail := bytes.IndexByte(src[end:], '\n'); tail >= 0 {
		line = append(line, src[end:end+tail]...)
	} else {
		line = append(line, src[end:]...)
	}
	return linelen.CheckLine(line)
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
