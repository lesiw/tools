// Package ifelsechain provides an analysis.Analyzer that reports an if-else
// chain long enough to read as a switch.
//
// Effective Go makes the case directly: a chain of else-if is a switch
// written as ifs, and a switch puts every condition at one indent with a
// single body each. Like go-critic's ifElseChain.
package ifelsechain

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ifelsechain",
	Doc:  "reports an if-else chain that reads as a switch",
	Run:  run,
}

// minChain is the number of branches a chain needs before it reads as a
// switch: the first if, each else-if, and a final else.
const minChain = 4

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			inner  = make(map[*ast.IfStmt]struct{})
			chains []*ast.IfStmt
		)
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if _, done := inner[stmt]; !ok || done {
				return true
			}
			chain := chain(stmt)
			for _, link := range chain[1:] {
				inner[link] = struct{}{}
			}
			if branches(chain) >= minChain && switchable(chain) {
				chains = append(chains, stmt)
			}
			return true
		})
		for _, stmt := range chains {
			pass.Reportf(stmt.Pos(),
				"if-else chain of %d branches; use a switch",
				branches(chain(stmt)),
			)
		}
	}
	return nil, nil
}

// chain returns the if statements of the chain stmt starts, outermost first.
func chain(stmt *ast.IfStmt) []*ast.IfStmt {
	out := []*ast.IfStmt{stmt}
	for {
		next, ok := stmt.Else.(*ast.IfStmt)
		if !ok {
			return out
		}
		out = append(out, next)
		stmt = next
	}
}

// branches returns how many branches the chain holds: one per if and else-if,
// and one for a final else.
func branches(chain []*ast.IfStmt) int {
	n := len(chain)
	if _, ok := chain[len(chain)-1].Else.(*ast.BlockStmt); ok {
		n++
	}
	return n
}

// switchable reports whether every link of the chain can be a case. An if with
// an init statement has no case form.
func switchable(chain []*ast.IfStmt) bool {
	for _, link := range chain {
		if link.Init != nil {
			return false
		}
	}
	return true
}
