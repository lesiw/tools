// Package underef provides an analysis.Analyzer that reports a redundant
// dereference in front of a selector.
//
// The Selectors section of the spec says that for a value x of type T or *T,
// where T is not a pointer or interface type, x.f denotes the field or method
// at the shallowest depth in T. It also says that for a defined pointer type
// x, x.f is shorthand for (*x).f when f is a field. So the dereference adds
// nothing, unless f is a method whose receiver type is the type of x: there
// (*x).f() panics on a nil x and x.f() calls f with a nil receiver. The README
// catalogs every exemption.
//
// The rule is adapted from go-critic's underef.
package underef

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "underef",
	Doc:  "reports a redundant dereference in front of a selector",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				check(pass, sel)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, sel *ast.SelectorExpr) {
	star, parens := deref(sel.X)
	if star == nil {
		return
	}
	operand, inner := strip(star.X)
	parens = append(parens, inner...)
	if !redundant(pass, operand, sel.Sel.Name) {
		return
	}
	outer := parens[0]
	diagnostic := analysis.Diagnostic{
		Pos:     outer.Pos(),
		End:     outer.End(),
		Message: "redundant dereference in front of a selector",
	}
	// A line break inside the parentheses is left alone: the operand would
	// start a line of its own, which ends the statement before it. Only a
	// break in code has that effect, but the check does not tell the two
	// apart.
	first := pass.Fset.Position(outer.Pos())
	if last := pass.Fset.Position(outer.End()); first.Line == last.Line {
		edits := []analysis.TextEdit{{Pos: star.Star, End: star.Star + 1}}
		if !needsParens(operand) {
			for _, paren := range parens {
				left, right := paren.Lparen, paren.Rparen
				edits = append(edits,
					analysis.TextEdit{Pos: left, End: left + 1},
					analysis.TextEdit{Pos: right, End: right + 1},
				)
			}
		}
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Drop the dereference",
			TextEdits: edits,
		}}
	}
	pass.Report(diagnostic)
}

// deref returns the dereference e holds behind any number of parentheses, and
// those parentheses.
func deref(e ast.Expr) (*ast.StarExpr, []*ast.ParenExpr) {
	expr, parens := strip(e)
	if star, ok := expr.(*ast.StarExpr); ok {
		return star, parens
	}
	return nil, parens
}

// strip returns e with its parentheses removed, and those parentheses.
func strip(e ast.Expr) (ast.Expr, []*ast.ParenExpr) {
	var parens []*ast.ParenExpr
	for {
		paren, ok := e.(*ast.ParenExpr)
		if !ok {
			return e, parens
		}
		parens = append(parens, paren)
		e = paren.X
	}
}

// redundant reports whether x.f means what (*x).f means, so the star can go.
func redundant(pass *analysis.Pass, operand ast.Expr, name string) bool {
	tv, ok := pass.TypesInfo.Types[operand]
	if !ok {
		return false
	}
	// A method expression stands in front of a type: (*T).M takes a *T, and
	// T.M takes a T.
	if tv.IsType() {
		return false
	}
	// A type parameter's selectors come from its constraint, so p.f() can call
	// a pointer-receiver method without the nil check (*p).f() performs.
	if _, ok := tv.Type.(*types.TypeParam); ok {
		return false
	}
	// The lookup finds no object where the shorthand would not compile.
	obj, _, _ := types.LookupFieldOrMethod(tv.Type, false, pass.Pkg, name)
	if obj == nil {
		return false
	}
	// The dereference is the nil check when the method's receiver type is the
	// operand's: (*p).M() panics when p is nil, and p.M() calls M with a nil
	// receiver.
	fn, isMethod := obj.(*types.Func)
	if !isMethod {
		return true
	}
	return !types.Identical(fn.Signature().Recv().Type(), tv.Type)
}

// needsParens reports whether the parentheses must survive the fix: a selector
// binds tighter than a unary operator.
func needsParens(operand ast.Expr) bool {
	switch operand.(type) {
	case *ast.UnaryExpr, *ast.StarExpr:
		return true
	}
	return false
}
