// Package dupelseif provides an analysis.Analyzer that reports an else-if
// whose condition repeats an earlier condition in the same chain.
//
// By the if statements section of the spec
// (https://go.dev/ref/spec#If_statements), an else-if is an if statement in
// the else branch, so it is reached only after every earlier condition was
// false. A later condition that matches an earlier one is false too when each
// condition reads the same value both times and no init statement or condition
// that can read a different value lies between them, so the branch can never
// run.
//
// Report only: removing the branch would drop any comment inside it or leave a
// local or an import unused, and the intended condition is not in the source.
//
// The rule is inspired by staticcheck's SA4014.
package dupelseif

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupelseif",
	Doc:  "reports an else-if whose condition repeats an earlier one",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		links := make(map[*ast.IfStmt]struct{})
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if _, ok := links[stmt]; !ok {
				check(pass, stmt, links)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each condition in the chain that repeats an earlier one. It
// records the chain's else-if statements in links so the walk does not check
// the same chain again from each one.
func check(pass *analysis.Pass, root *ast.IfStmt, links map[*ast.IfStmt]struct{}) {
	var (
		seen = make(map[string]struct{})
		stmt = root
	)
	for {
		if stmt.Init != nil {
			clear(seen)
		}
		if !astexpr.Stable(pass, stmt.Cond) {
			clear(seen)
		} else {
			key := astexpr.Text(ast.Unparen(stmt.Cond))
			if _, ok := seen[key]; ok {
				pass.Report(analysis.Diagnostic{
					Pos:     stmt.Cond.Pos(),
					End:     stmt.Cond.End(),
					Message: "else-if repeats an earlier condition; the branch can never run",
				})
			}
			seen[key] = struct{}{}
		}
		next, ok := stmt.Else.(*ast.IfStmt)
		if !ok {
			return
		}
		links[next] = struct{}{}
		stmt = next
	}
}
