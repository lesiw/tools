// Package unreachablecase provides an analysis.Analyzer that reports a type in
// a type switch case that an earlier interface case matches.
//
// Cases are tested in source order, and a case with an interface type matches
// every value whose type implements it. The case for a later type that
// implements that interface never runs. An earlier type parameter is not
// treated as an interface case.
//
// No fix is offered. Moving the case before the earlier one makes it
// reachable, which changes behavior, and where it belongs is a choice outside
// the rule. Removing a type that is alone in its clause would move the body.
// Several reported types can appear in one clause, and a comment can follow
// any type in a multi-type clause, so one fix would overlap another or delete
// text.
//
// Adapted from staticcheck's SA4020.
package unreachablecase

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unreachablecase",
	Doc:  "reports a case type that an earlier interface case matches",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.TypeSwitchStmt); ok {
				check(pass, stmt)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each case type that an earlier interface case matches.
func check(pass *analysis.Pass, stmt *ast.TypeSwitchStmt) {
	var earlier []types.Type
	for _, node := range stmt.Body.List {
		clause := node.(*ast.CaseClause)
		if len(clause.List) == 0 {
			continue // a default clause lists no type
		}
		var current []types.Type
		for _, expr := range clause.List {
			typ := pass.TypesInfo.TypeOf(expr)
			if typ == nil || typ == types.Typ[types.UntypedNil] {
				continue
			}
			if first, ok := matched(earlier, typ); ok {
				pass.Reportf(
					expr.Pos(),
					"unreachable type in case: %s already matches every %s; an earlier case is tested first",
					name(first, pass),
					name(typ, pass),
				)
			}
			current = append(current, typ)
		}
		// A case lists alternatives, so no type in it comes before another.
		earlier = append(earlier, current...)
	}
}

// matched returns the first earlier type that an interface case matches typ
// through.
func matched(earlier []types.Type, typ types.Type) (types.Type, bool) {
	for _, t := range earlier {
		if subsumes(t, typ) {
			return t, true
		}
	}
	return nil, false
}

// subsumes reports whether a case with type t matches every value of type v.
// Only an interface does that, and it does so for the types that implement it.
func subsumes(t, v types.Type) bool {
	if isTypeParam(t) {
		return false
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return types.Implements(v, iface)
}

// isTypeParam reports whether t is a type parameter. An earlier type parameter
// is not treated as an interface case.
func isTypeParam(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}

// name returns t with a qualifier for each imported package.
func name(t types.Type, pass *analysis.Pass) string {
	return types.TypeString(t, types.RelativeTo(pass.Pkg))
}
