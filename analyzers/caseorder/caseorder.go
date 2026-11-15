// Package caseorder provides an analysis.Analyzer that reports a switch whose
// case values are a contiguous run in neither ascending nor descending order.
//
// A reader has no place to start in a switch that runs 3, 1, 2, so they read
// every clause. Literal cases with no fallthrough behave the same in any
// order, so the order is presentational: this is a house rule, and the suite
// writes a run of values in one direction.
//
// The name comes from go-critic's caseOrder, which reports a type switch
// clause that can never run. The README catalogs every exemption.
//
// There is no fix: a clause's leading comment is outside the clause, so a
// rewrite would drop it or guess which clause it belongs to.
package caseorder

import (
	"go/ast"
	"go/constant"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "caseorder",
	Doc:  "reports a switch whose case values are a contiguous run in neither ascending nor descending order",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.SwitchStmt); ok {
				checkSwitch(pass, stmt)
			}
			return true
		})
	}
	return nil, nil
}

func checkSwitch(pass *analysis.Pass, stmt *ast.SwitchStmt) {
	values, ok := valuesOf(pass, stmt)
	if !ok || !contiguous(values) || ordered(values) {
		return
	}
	pass.Reportf(
		stmt.Switch,
		"case values are in neither ascending nor descending order",
	)
}

// valuesOf returns the values a switch's clauses name, in the order they are
// written, and reports false for a switch the rule leaves alone.
func valuesOf(pass *analysis.Pass, stmt *ast.SwitchStmt) ([]int64, bool) {
	var values []int64
	for i, node := range stmt.Body.List {
		clause := node.(*ast.CaseClause)
		// The clause order is part of the program.
		if fallsThrough(clause) {
			return nil, false
		}
		if len(clause.List) == 0 {
			// A default in the middle splits the switch into two runs.
			if i != 0 && i != len(stmt.Body.List)-1 {
				return nil, false
			}
			continue
		}
		if len(clause.List) > 1 {
			// The clause covers a set of values at once, so it has no single
			// place in the run.
			return nil, false
		}
		n, ok := literal(pass, clause.List[0])
		if !ok {
			return nil, false
		}
		values = append(values, n)
	}
	return values, true
}

// literal returns the value an integer literal states, and reports false for
// any other expression.
func literal(pass *analysis.Pass, expr ast.Expr) (int64, bool) {
	lit, ok := ast.Unparen(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	// A floating-point tag records the literal as a Float, and Int64Val takes
	// an Int.
	return constant.Int64Val(constant.ToInt(pass.TypesInfo.Types[lit].Value))
}

// fallsThrough reports whether the clause ends in a fallthrough, which may
// carry a label of its own.
func fallsThrough(clause *ast.CaseClause) bool {
	if len(clause.Body) == 0 {
		return false
	}
	last := clause.Body[len(clause.Body)-1]
	if label, ok := last.(*ast.LabeledStmt); ok {
		last = label.Stmt
	}
	branch, ok := last.(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

// contiguous reports whether the values are a run. Two clauses cannot name the
// same value, so distinct values with a span as wide as the list leave no gap.
func contiguous(values []int64) bool {
	if len(values) < 2 {
		return true
	}
	lo, hi := values[0], values[0]
	for _, n := range values[1:] {
		lo, hi = min(lo, n), max(hi, n)
	}
	return hi-lo+1 == int64(len(values))
}

// ordered reports whether the values are ascending or descending.
func ordered(values []int64) bool {
	var rises int
	for i := 1; i < len(values); i++ {
		if values[i] > values[i-1] {
			rises++
		}
	}
	return rises == 0 || rises == len(values)-1
}
