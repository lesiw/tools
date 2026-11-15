// Package badcond provides an analysis.Analyzer that reports a condition that
// is always true or always false.
//
// n < 0 && n > 10 asks for a value below 0 and above 10, so no value satisfies
// it. Adapted from go-critic's badCond: the contradiction rule adds the
// disjunction that covers every value, and the loop rule reports only a
// condition the init statement makes false. The README catalogs every check
// and exemption.
//
// A for clause's init statement runs before its condition is first tested, so
// a condition the init statement makes false is false at the first test, and
// the body never runs.
//
// The comparison operators section of the spec compares floating-point values
// "as defined by the IEEE 754 standard", which is why a tautology of ordering
// comparisons needs an operand type that cannot hold a NaN.
//
// There is no fix: the tool cannot tell which condition was meant.
package badcond

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "badcond",
	Doc:  "reports a condition that is always true or always false",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ForStmt:
				checkCond(pass, node)
			case *ast.BinaryExpr:
				message, ok := settled(pass, node)
				if !ok {
					return true
				}
				pass.Report(analysis.Diagnostic{
					Pos:     node.Pos(),
					End:     node.End(),
					Message: message,
				})
				return false // a pair inside a pair is the same typo
			}
			return true
		})
	}
	return nil, nil
}

// bound is one comparison of a value against a constant.
type bound struct {
	x  ast.Expr
	op token.Token
	c  constant.Value
	ct types.Type // the type the constant was converted to
}

// below reports whether the comparison admits only values below its constant.
func (b bound) below() bool { return b.op == token.LSS || b.op == token.LEQ }

// above reports whether the comparison admits only values above its constant.
func (b bound) above() bool { return b.op == token.GTR || b.op == token.GEQ }

// strict reports whether the operator excludes its constant.
func (b bound) strict() bool { return b.op == token.GTR || b.op == token.LSS }

// settled returns the diagnostic a chain of comparisons settles on, when it
// settles anything.
func settled(pass *analysis.Pass, chain *ast.BinaryExpr) (string, bool) {
	if chain.Op != token.LAND && chain.Op != token.LOR {
		return "", false
	}
	var (
		terms  = flatten(chain, chain.Op)
		bounds = make([]*bound, len(terms))
	)
	for i, term := range terms {
		if b, ok := comparison(pass, term); ok {
			bounds[i] = &b
		}
	}
	for i, first := range bounds {
		if first == nil {
			continue
		}
		text := astexpr.Text(first.x)
		for j := i + 1; j < len(bounds); j++ {
			// Any effect between the two comparisons can change the value
			// they read, so the pair no longer says anything about it.
			if astexpr.Effectful(pass, terms[j-1]) {
				break
			}
			second := bounds[j]
			if second == nil || astexpr.Text(second.x) != text {
				continue
			}
			if chain.Op == token.LAND {
				if unsatisfiable(*first, *second) {
					return "condition is always false", true
				}
				continue
			}
			if total(pass, *first, *second) {
				return "condition is always true", true
			}
		}
	}
	return "", false
}

// unsatisfiable reports whether no value satisfies both comparisons.
func unsatisfiable(a, b bound) bool {
	if negated(a, b) {
		return true
	}
	lower, upper, ok := sides(a, b)
	return ok && apart(lower, upper)
}

// total reports whether every value satisfies one of the two comparisons.
func total(pass *analysis.Pass, a, b bound) bool {
	// == and != negate each other, so one of the pair is true for every value,
	// a NaN included.
	if negated(a, b) {
		return true
	}
	if astexpr.MayBeNaN(pass.TypesInfo.TypeOf(a.x)) {
		return false
	}
	lower, upper, ok := sides(a, b)
	return ok && joined(lower, upper)
}

// comparison returns a comparison of an operand with a constant. The ordering
// operators give the bounds that sides pairs up. An equality comparison
// contributes only to negated.
func comparison(pass *analysis.Pass, e ast.Expr) (bound, bool) {
	bin, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok {
		return bound{}, false
	}
	op := bin.Op
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
	default:
		return bound{}, false
	}
	x, c := ast.Unparen(bin.X), ast.Unparen(bin.Y)
	xv, xConst := constantValue(pass, x)
	cv, cConst := constantValue(pass, c)
	switch {
	case !xConst && !cConst:
		return bound{}, false
	case xConst && !cConst:
		x, c, cv, op = c, x, xv, reverse(op)
	}
	if astexpr.Effectful(pass, x) {
		return bound{}, false
	}
	return bound{x: x, op: op, c: cv, ct: pass.TypesInfo.TypeOf(c)}, true
}

// negated reports whether one comparison is the equality the other denies.
func negated(a, b bound) bool {
	if a.ct == nil || !types.Identical(a.ct, b.ct) {
		return false
	}
	switch {
	case a.op == token.EQL && b.op == token.NEQ:
	case a.op == token.NEQ && b.op == token.EQL:
	default:
		return false
	}
	return constant.Compare(a.c, token.EQL, b.c)
}

// sides returns the pair with its lower bound first, when the two comparisons
// point at each other. Two bounds on one side leave a range that neither of
// them rules out.
func sides(a, b bound) (lower, upper bound, ok bool) {
	switch {
	case a.above() && b.below():
		return a, b, true
	case b.above() && a.below():
		return b, a, true
	}
	return bound{}, bound{}, false
}

// apart reports whether the lower bound sits above the upper one, so that no
// value satisfies both.
func apart(lower, upper bound) bool {
	if constant.Compare(lower.c, token.GTR, upper.c) {
		return true
	}
	if constant.Compare(lower.c, token.LSS, upper.c) {
		return false
	}
	return lower.strict() || upper.strict()
}

// joined reports whether the bounds meet, so that every value satisfies one of
// the two.
func joined(lower, upper bound) bool {
	if constant.Compare(lower.c, token.LSS, upper.c) {
		return true
	}
	if constant.Compare(lower.c, token.GTR, upper.c) {
		return false
	}
	return !lower.strict() || !upper.strict()
}

// checkCond reports a for clause whose condition is false at the first test,
// before the body can run.
func checkCond(pass *analysis.Pass, loop *ast.ForStmt) {
	name, value, ok := initOf(loop)
	if !ok || loop.Cond == nil {
		return
	}
	start, ok := constantValue(pass, value)
	if !ok {
		return
	}
	// The pair rule reports this chain when the walk reaches it, so the
	// condition would only repeat it.
	if chain, ok := ast.Unparen(loop.Cond).(*ast.BinaryExpr); ok {
		if _, ok := settled(pass, chain); ok {
			return
		}
	}
	declared := pass.TypesInfo.ObjectOf(name)
	// A conjunction is false when any one of its terms is, as long as every
	// term before it is free of effects.
	for _, term := range flatten(loop.Cond, token.LAND) {
		if astexpr.Effectful(pass, term) {
			return
		}
		guard, ok := comparison(pass, term)
		if !ok {
			continue
		}
		// The term has to read the variable the init statement assigned.
		ident, ok := guard.x.(*ast.Ident)
		if !ok || pass.TypesInfo.ObjectOf(ident) != declared {
			continue
		}
		if constant.Compare(start, guard.op, guard.c) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:     term.Pos(),
			End:     term.End(),
			Message: "loop condition is false before the first iteration",
		})
		return
	}
}

// initOf returns the variable a for clause's init statement assigns and the
// expression it gives it.
func initOf(loop *ast.ForStmt) (*ast.Ident, ast.Expr, bool) {
	init, ok := loop.Init.(*ast.AssignStmt)
	if !ok {
		return nil, nil, false
	}
	if init.Tok != token.DEFINE && init.Tok != token.ASSIGN {
		return nil, nil, false
	}
	if len(init.Lhs) != 1 || len(init.Rhs) != 1 {
		return nil, nil, false
	}
	name, ok := init.Lhs[0].(*ast.Ident)
	return name, init.Rhs[0], ok
}

// constantValue returns the value of a constant expression.
func constantValue(pass *analysis.Pass, e ast.Expr) (constant.Value, bool) {
	v := pass.TypesInfo.Types[e].Value
	return v, v != nil
}

// flatten returns the operands of a chain of op.
func flatten(e ast.Expr, op token.Token) []ast.Expr {
	b, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || b.Op != op {
		return []ast.Expr{ast.Unparen(e)}
	}
	return append(flatten(b.X, op), flatten(b.Y, op)...)
}

// reverse returns the operator that reads the same comparison with its
// operands swapped.
func reverse(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.LEQ:
		return token.GEQ
	case token.GTR:
		return token.LSS
	case token.GEQ:
		return token.LEQ
	}
	return op
}
