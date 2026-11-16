// Package deferinloop provides an analysis.Analyzer that reports a deferred
// call in a loop body.
//
// The specification's Defer statements section says a deferred call runs
// immediately before the surrounding function returns, in the reverse order
// they were deferred, so a loop iteration that ends without returning leaves
// its call for the function. Each iteration adds another call, and whatever
// the loop acquired stays held until the function returns.
//
// The rule reports without a fix: a call moved out of the defer no longer runs
// on the paths that leave the function, so the edit is the author's. The
// README catalogs the check and its exemptions.
//
// The rule is like go-critic's deferInLoop, narrowed to the loop bodies where
// the deferral accumulates.
package deferinloop

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
)

var Analyzer = &analysis.Analyzer{
	Name:     "deferinloop",
	Doc:      "reports a deferred call in a loop body",
	Requires: []*analysis.Analyzer{ctrlflow.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	cfgs := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	for _, file := range pass.Files {
		// A test releases each case's cleanup through a defer, so that
		// t.Fatal, which ends the test from anywhere, still runs it.
		name := pass.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				check(pass, cfgs.FuncDecl(n))
			case *ast.FuncLit:
				check(pass, cfgs.FuncLit(n))
			}
			return true
		})
	}
	return nil, nil
}

// check reports every defer in g that control can reach again through a loop,
// one call per iteration waiting at the function's return.
func check(pass *analysis.Pass, g *cfg.CFG) {
	if g == nil {
		return // a declaration with no body, as an assembly stub has
	}
	for _, b := range g.Blocks {
		if !b.Live || !loopsBack(b) {
			continue
		}
		for _, node := range b.Nodes {
			if d, ok := node.(*ast.DeferStmt); ok {
				pass.Report(analysis.Diagnostic{
					Pos:     d.Pos(),
					Message: "deferred calls in a loop accumulate until the function returns",
				})
			}
		}
	}
}

// loopsBack reports whether control can leave b and arrive at b again through
// a loop's back edge. A body that leaves the loop one way or another never
// accumulates: the deferred calls run at the return that ends it.
func loopsBack(b *cfg.Block) bool {
	type state struct {
		block *cfg.Block
		loop  bool
	}
	var (
		seen  = map[state]struct{}{}
		queue = make([]state, 0, len(b.Succs))
	)
	for _, succ := range b.Succs {
		queue = append(queue, state{succ, isLoop(succ)})
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if s.block == b {
			if s.loop {
				return true
			}
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		for _, succ := range s.block.Succs {
			queue = append(queue, state{succ, s.loop || isLoop(succ)})
		}
	}
	return false
}

// isLoop reports whether b is a loop body, which is the block every back edge
// into the loop passes through.
func isLoop(b *cfg.Block) bool {
	switch b.Kind {
	case cfg.KindForBody, cfg.KindRangeBody:
		return true
	}
	return false
}
