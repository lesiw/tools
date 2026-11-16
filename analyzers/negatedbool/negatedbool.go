// Package negatedbool provides an analysis.Analyzer that reports a boolean
// expression that a negation makes constant or redundant.
//
// The logical operators section of the spec defines !x as the inverse of x,
// x && y as false where either operand is false, and x || y as true where
// either operand is true (https://go.dev/ref/spec#Logical_operators). The same
// section says logical operators yield a result of the same type as their
// operands, so writing x in place of !!x preserves a defined boolean type or a
// type parameter.
//
// A conjunction or disjunction is reported when one immediate operand is a
// negation and the other is a name, a constant expression, or a negation of
// one with the same text. A larger operand is left alone, even when it looks
// stable. A double negation reads its operand once and is reported whatever
// the effect.
//
// The fix writes the operand in place of a double negation. A conjunction or
// disjunction is fixed to its constant value only when the operand is a
// constant with no variable reference or a parameter or result variable, its
// type is the predeclared bool, an alias of it, or an untyped boolean, and the
// replacement is the predeclared false or true in scope there. A local
// variable is report-only because dropping its last read would make it unused.
// A package variable is report-only because the edit can reorder package
// initialization when the expression appears in an initializer. A defined
// boolean type is report-only because an untyped constant can change generic
// inference and short declaration types. A constant from another package is
// report-only so the edit does not leave an import unused.
//
// The rule is adapted from staticcheck's SA4013, which reports a double
// negation.
package negatedbool

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "negatedbool",
	Doc:  "reports a boolean expression that a negation makes constant or redundant",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.UnaryExpr:
				if cancels(pass, file, n) {
					return false
				}
			case *ast.BinaryExpr:
				if contradicts(pass, file, n) {
					return false
				}
			}
			// A fix covers any report inside it, so a nested report waits
			// for the next round.
			return true
		})
	}
	return nil, nil
}

// cancels reports a double negation and whether it left a fix.
func cancels(pass *analysis.Pass, file *ast.File, outer *ast.UnaryExpr) bool {
	if outer.Op != token.NOT {
		return false
	}
	inner, ok := ast.Unparen(outer.X).(*ast.UnaryExpr)
	if !ok || inner.Op != token.NOT {
		return false
	}
	diag := analysis.Diagnostic{
		Pos:     outer.Pos(),
		End:     outer.End(),
		Message: "negating a boolean twice has no effect",
	}
	// The operand replaces the pair, and the replacement keeps its
	// parentheses. A statement header cannot begin with a composite literal,
	// so one more pair of parentheses keeps it parseable there.
	text := astexpr.Text(inner.X)
	if startsComposite(inner.X) {
		text = "(" + text + ")"
	}
	text = replacement(pass, outer.Pos(), text)
	if fits(pass, outer, text) && !commented(file, outer.Pos(), outer.End()) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Write the operand",
			TextEdits: []analysis.TextEdit{{
				Pos:     outer.Pos(),
				End:     outer.End(),
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
	return len(diag.SuggestedFixes) > 0
}

// contradicts reports a conjunction or disjunction whose immediate operands
// negate each other, and whether it left a fix.
func contradicts(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) bool {
	if bin.Op != token.LAND && bin.Op != token.LOR {
		return false
	}
	operand, ok := negatedSide(bin)
	if !ok || !plain(pass, operand) {
		return false
	}
	answer := "false"
	message := "the conjunction of an expression and its negation is always false"
	fixMessage := "Replace with false"
	if bin.Op == token.LOR {
		answer = "true"
		message = "the disjunction of an expression and its negation is always true"
		fixMessage = "Replace with true"
	}
	diag := analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: message,
	}
	text := replacement(pass, bin.Pos(), answer)
	if fixable(pass, file, bin, operand, text) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: fixMessage,
			TextEdits: []analysis.TextEdit{{
				Pos:     bin.Pos(),
				End:     bin.End(),
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
	return len(diag.SuggestedFixes) > 0
}

// negatedSide returns the operand of the negation in bin when the other
// immediate operand reads the same value.
func negatedSide(bin *ast.BinaryExpr) (ast.Expr, bool) {
	for _, side := range [][2]ast.Expr{{bin.X, bin.Y}, {bin.Y, bin.X}} {
		negation, ok := ast.Unparen(side[0]).(*ast.UnaryExpr)
		if !ok || negation.Op != token.NOT {
			continue
		}
		if sameText(negation.X, side[1]) {
			return negation.X, true
		}
	}
	return nil, false
}

// plain reports whether e is a name, a constant expression, or a negation of
// one.
func plain(pass *analysis.Pass, e ast.Expr) bool {
	e = ast.Unparen(e)
	if pass.TypesInfo.Types[e].Value != nil {
		return true
	}
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		return plain(pass, u.X)
	}
	ident, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	switch pass.TypesInfo.ObjectOf(ident).(type) {
	case *types.Var, *types.Const:
		return true
	}
	return false
}

// fixable reports whether bin can be replaced with text.
func fixable(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, operand ast.Expr, text string) bool {
	var (
		constant = pass.TypesInfo.Types[ast.Unparen(operand)].Value != nil
		typ      = pass.TypesInfo.TypeOf(bin)
	)
	if constant {
		if hasVar(pass, operand) {
			return false
		}
		typ = pass.TypesInfo.TypeOf(operand)
	}
	if !isBool(typ) || imported(pass, operand) {
		return false
	}
	answer := "false"
	if bin.Op == token.LOR {
		answer = "true"
	}
	name := imports.Lookup(pass, answer, bin.Pos())
	if name != types.Universe.Lookup(answer) {
		return false
	}
	if pass.TypesInfo.Types[ast.Unparen(operand)].Value == nil {
		ident, ok := ast.Unparen(operand).(*ast.Ident)
		if !ok {
			return false
		}
		v, ok := pass.TypesInfo.ObjectOf(ident).(*types.Var)
		if !ok || v.Kind() != types.ParamVar && v.Kind() != types.ResultVar {
			return false
		}
	}
	return fits(pass, bin, text) && !commented(file, bin.Pos(), bin.End())
}

// hasVar reports whether e names a variable.
func hasVar(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := pass.TypesInfo.Uses[ident].(*types.Var); ok {
			found = true
		}
		return !found
	})
	return found
}

// imported reports whether e names an object from another package.
func imported(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := pass.TypesInfo.Uses[ident]
		if obj != nil && obj.Pkg() != nil && obj.Pkg() != pass.Pkg {
			found = true
		}
		return !found
	})
	return found
}

// replacement returns text with a space before it when the byte before pos
// would otherwise join it to the previous token.
func replacement(pass *analysis.Pass, pos token.Pos, text string) string {
	file := pass.Fset.File(pos)
	if file == nil {
		return text
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return text
	}
	off := file.Offset(pos)
	if off <= 0 {
		return text
	}
	b := src[off-1]
	switch {
	case b == '_', b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z',
		b >= '0' && b <= '9', b >= 0x80:
		return " " + text
	}
	return text
}

// fits reports whether text keeps the line the expression sits on within the
// width limit.
func fits(pass *analysis.Pass, expr ast.Expr, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	file := pass.Fset.File(expr.Pos())
	if file == nil {
		return false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	start := file.Offset(expr.Pos())
	end := file.Offset(expr.End())
	begin := bytes.LastIndexByte(src[:start], '\n') + 1
	line := append(slices.Clone(src[begin:start]), text...)
	if tail := bytes.IndexByte(src[end:], '\n'); tail >= 0 {
		line = append(line, src[end:end+tail]...)
	} else {
		line = append(line, src[end:]...)
	}
	return linelen.CheckLine(line)
}

// startsComposite reports whether e reaches a composite literal through its
// prefix operators, selectors, calls, and similar.
func startsComposite(e ast.Expr) bool {
	for {
		switch n := e.(type) {
		case *ast.StarExpr:
			e = n.X
		case *ast.UnaryExpr:
			e = n.X
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

// sameText reports whether a and b print as the same expression, ignoring
// parentheses around either one. Equal text gives the same value.
func sameText(a, b ast.Expr) bool {
	return astexpr.Text(ast.Unparen(a)) == astexpr.Text(ast.Unparen(b))
}

// isBool reports whether t is bool or the type of an untyped boolean value.
func isBool(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
