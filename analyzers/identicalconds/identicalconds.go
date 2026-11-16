// Package identicalconds provides an analysis.Analyzer that reports an
// else-if that repeats an earlier condition in the same chain.
//
// The Go spec says an if statement executes its if branch when the expression
// is true and, when present, its else branch otherwise
// (https://go.dev/ref/spec#If_statements). Every else-if is reached through
// the else branch, so the conditions before it are false there. A repeat is
// reported only when no init statement or value-changing condition lies
// between the two. The earlier condition is false at the later test, and
// nothing between them can change the value it read, so the repeated condition
// is false too and the branch can never run.
//
// An init statement ends the comparison: it can change a variable that an
// earlier condition used, or declare a name that shadows one, so conditions
// before it are not compared with conditions after it. A call that is not pure
// or that has an effectful argument or operand ends the comparison, and so
// does a channel receive. The README covers the exemptions.
//
// Report only: removing the branch would drop any comment inside it, and a
// repeated condition usually means one condition was mistyped. The intended
// condition is not in the source.
//
// The rule is adapted from revive's identical-ifelseif-conditions.
package identicalconds

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "identicalconds",
	Doc:  "reports an else-if that repeats an earlier condition in the same chain",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		elseIf := make(map[*ast.IfStmt]struct{})
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if next, ok := stmt.Else.(*ast.IfStmt); ok {
				elseIf[next] = struct{}{}
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if _, ok := elseIf[stmt]; ok {
				return true
			}
			check(pass, stmt)
			return true
		})
	}
	return nil, nil
}

// check reports each condition in the chain that repeats an earlier one.
func check(pass *analysis.Pass, root *ast.IfStmt) {
	seen := make(map[string]struct{})
	for stmt := root; stmt != nil; {
		if stmt.Init != nil {
			clear(seen)
		}
		cond := ast.Unparen(stmt.Cond)
		if effectful(pass, cond) {
			clear(seen)
		} else {
			key := astexpr.Text(cond)
			if _, ok := seen[key]; ok {
				pass.Reportf(
					cond.Pos(),
					"else-if repeats an earlier condition; this branch can never run",
				)
			}
			seen[key] = struct{}{}
		}
		next, ok := stmt.Else.(*ast.IfStmt)
		if !ok {
			break
		}
		stmt = next
	}
}

// effectful reports whether reading e twice can produce a different result. A
// function literal runs no code when it is created, so its body is skipped.
func effectful(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if !astexpr.Pure(pass, n) || lenOfChannel(pass, n) {
				found = true
			}
		case *ast.UnaryExpr:
			switch n.Op {
			case token.ARROW:
				found = true
			case token.AND:
				found = hasCompositeLit(n.X)
			}
		}
		return !found
	})
	return found
}

// lenOfChannel reports whether call is len of a value that can be a channel.
func lenOfChannel(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != types.Universe.Lookup("len") {
		return false
	}
	if len(call.Args) == 0 {
		return false
	}
	return mayBeChannel(pass.TypesInfo.TypeOf(call.Args[0]))
}

// hasCompositeLit reports whether e contains a composite literal.
func hasCompositeLit(e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.CompositeLit); ok {
			found = true
		}
		return !found
	})
	return found
}

// mayBeChannel reports whether t can be a channel.
func mayBeChannel(t types.Type) bool {
	if t == nil {
		return false
	}
	if param, ok := t.(*types.TypeParam); ok {
		t = param.Constraint()
	}
	var (
		underlying = t.Underlying()
		iface, ok  = underlying.(*types.Interface)
	)
	if !ok {
		_, ok := underlying.(*types.Chan)
		return ok
	}
	for embedded := range iface.EmbeddedTypes() {
		if union, ok := embedded.(*types.Union); ok {
			for term := range union.Terms() {
				if mayBeChannel(term.Type()) {
					return true
				}
			}
			continue
		}
		if mayBeChannel(embedded) {
			return true
		}
	}
	return false
}
