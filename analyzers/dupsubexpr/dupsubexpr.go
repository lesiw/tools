// Package dupsubexpr provides an analysis.Analyzer that reports an operator
// whose operands are the same value.
//
// items[k] < items[k] compares one item with itself, so one of the two
// operands was meant to name something else. The rule is like go-critic's
// dupSubExpr, with a narrower set of operators and a wider NaN exemption; the
// README catalogs every check and exemption.
//
// The comparison operators section of the spec compares floating point "as
// defined by the IEEE 754 standard". Under that standard a NaN is unequal to
// itself, which is why an operand that can hold one is left alone.
//
// There is no fix: the tool cannot tell which operand was meant.
package dupsubexpr

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupsubexpr",
	Doc:  "reports an operator whose operands are the same value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			expr, ok := n.(*ast.BinaryExpr)
			if !ok || !repeated(pass, expr) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     expr.Pos(),
				End:     expr.End(),
				Message: "both operands are the same value",
			})
			return false // a repeat inside a repeat is the same typo
		})
	}
	return nil, nil
}

// repeated reports whether expr applies its operator to one expression twice.
func repeated(pass *analysis.Pass, expr *ast.BinaryExpr) bool {
	var nanMatters bool
	switch expr.Op {
	case token.EQL, token.NEQ, token.LEQ, token.GEQ, token.QUO, token.SUB:
		nanMatters = true // a NaN changes each of these answers
	case token.LSS, token.GTR, token.REM:
		// < and > are false whether or not a NaN is involved, and % takes no
		// float.
	default:
		return false
	}
	x, y := ast.Unparen(expr.X), ast.Unparen(expr.Y)
	// Equal text is one expression, so one effect check answers for both.
	if text(x) != text(y) || effectful(pass, x) {
		return false
	}
	if nanMatters {
		// f != f can be a NaN test, and x's type stands for y's. A constant is
		// never a NaN, so 1.0 == 1.0 still reports.
		operand := pass.TypesInfo.Types[x]
		if operand.Value == nil && mayBeNaN(operand.Type) {
			return false
		}
	}
	// A constant expression is left alone unless it is a bool: [2 / 2]byte is
	// one byte, and 1 == 1 reports.
	v := pass.TypesInfo.Types[expr].Value
	return v == nil || v.Kind() == constant.Bool
}

// text returns e printed on one line. A fresh FileSet drops the source line
// breaks, so a wrapped operand matches an unwrapped one, and go/printer keeps
// the elements of a composite literal, which types.ExprString would elide.
func text(e ast.Expr) string {
	var buf bytes.Buffer
	// A bytes.Buffer cannot fail.
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
}

// effectful reports whether reading e can give a different value the second
// time.
func effectful(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if !pureCall(pass, n) {
				found = true
			}
		case *ast.UnaryExpr:
			switch n.Op {
			case token.ARROW:
				found = true // a receive takes an element
			case token.AND:
				// An address of a literal is a new value each time.
				found = found || hasLiteral(n.X)
			}
		}
		return !found
	})
	return found
}

// hasLiteral reports whether e contains a composite literal.
func hasLiteral(e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.CompositeLit); ok {
			found = true
		}
		return !found
	})
	return found
}

// pureCall reports whether evaluating call twice gives the same value. A
// conversion does, and so does a builtin that only reads its arguments.
func pureCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if tv, ok := pass.TypesInfo.Types[call.Fun]; ok && tv.IsType() {
		return true
	}
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	// The allowance is closed: a builtin added to the language counts as
	// effectful until it is listed here.
	switch pass.TypesInfo.Uses[ident] {
	case types.Universe.Lookup("cap"), types.Universe.Lookup("complex"),
		types.Universe.Lookup("imag"), types.Universe.Lookup("len"),
		types.Universe.Lookup("max"), types.Universe.Lookup("min"),
		types.Universe.Lookup("real"):
		return true
	}
	return false
}

// mayBeNaN reports whether a value of type t can be a NaN. An interface can
// hold one, and a type parameter is left to its constraint.
func mayBeNaN(t types.Type) bool {
	if param, ok := t.(*types.TypeParam); ok {
		return constraintNaN(param.Constraint())
	}
	switch t := t.Underlying().(type) {
	case *types.Basic:
		return t.Info()&(types.IsFloat|types.IsComplex) != 0
	case *types.Interface:
		return true
	case *types.Struct:
		for field := range t.Fields() {
			if mayBeNaN(field.Type()) {
				return true
			}
		}
	case *types.Array:
		return mayBeNaN(t.Elem())
	}
	return false
}

// constraintNaN reports whether the type set of c holds a type that can be a
// NaN. The elements of an interface are intersected, so one element that rules
// a NaN out settles the constraint.
func constraintNaN(c types.Type) bool {
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if mayBeNaN(term.Type()) {
				return true
			}
		}
		return false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return mayBeNaN(c)
	}
	for embedded := range iface.EmbeddedTypes() {
		if !constraintNaN(embedded) {
			return false
		}
	}
	// No element rules a NaN out, so any type with the methods fits.
	return true
}
