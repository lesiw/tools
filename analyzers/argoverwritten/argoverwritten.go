// Package argoverwritten provides an analysis.Analyzer that reports a function
// parameter whose passed value is never used because the body assigns to it.
//
// The Go spec initializes each parameter variable with the corresponding
// argument when the call begins (https://go.dev/ref/spec#Calls), and an
// assignment replaces the value stored in a variable
// (https://go.dev/ref/spec#Assignment_statements). An assignment to a
// parameter before the body uses it replaces the caller's value, so the caller
// computes and passes a value that the function never uses.
//
//	func reset(count int) int {
//		count = 0
//		return count
//	}
//
// The README catalogs every check and exemption.
//
// Report only: the repair names the parameter _, declares a local for the
// assigned value, drops the assignment, or removes the parameter, and the
// right one depends on the caller.
//
// The rule is adapted from staticcheck's SA4009.
package argoverwritten

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name:     "argoverwritten",
	Doc:      "reports a parameter whose passed value is never used",
	Requires: []*analysis.Analyzer{buildssa.Analyzer, ctrlflow.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		result = pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		seen   = make(map[*ssa.Function]struct{})
		visit  func(*ssa.Function)
	)
	visit = func(fn *ssa.Function) {
		if fn == nil {
			return
		}
		if _, ok := seen[fn]; ok {
			return
		}
		seen[fn] = struct{}{}
		check(pass, fn)
		for _, anon := range fn.AnonFuncs {
			visit(anon)
		}
	}
	for _, fn := range result.SrcFuncs {
		visit(fn)
	}
	// A function literal in a package-level declaration is created by the
	// synthetic init function, which is not one of the source functions.
	for _, member := range result.Pkg.Members {
		if fn, ok := member.(*ssa.Function); ok {
			visit(fn)
		}
	}
	return nil, nil
}

// check reports each named parameter of fn whose initial value nothing reads
// and that the body assigns.
func check(pass *analysis.Pass, fn *ssa.Function) {
	var (
		node = fn.Syntax()
		typ  *ast.FuncType
		body *ast.BlockStmt
	)
	switch fn := node.(type) {
	case *ast.FuncDecl:
		typ, body = fn.Type, fn.Body
	case *ast.FuncLit:
		typ, body = fn.Type, fn.Body
	default:
		return
	}
	if body == nil || typ.Params == nil {
		return
	}
	c := &checker{
		pass:  pass,
		body:  body,
		live:  liveSpans(pass, node),
		reach: liveBlocks(fn),
		byObj: make(map[types.Object]*ssa.Parameter, len(fn.Params)),
	}
	for _, param := range fn.Params {
		c.byObj[param.Object()] = param
	}
	for _, field := range typ.Params.List {
		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}
			param := c.byObj[pass.TypesInfo.Defs[name]]
			if param == nil || read(param, c.reach) {
				continue
			}
			if !c.assigned(name) {
				continue
			}
			msg := "the value passed as " + name.Name + " is never used"
			pass.Report(analysis.Diagnostic{
				Pos:     name.Pos(),
				End:     name.End(),
				Message: msg,
			})
		}
	}
}

// liveBlocks returns the blocks a path from the function's entry can reach.
func liveBlocks(fn *ssa.Function) map[*ssa.BasicBlock]bool {
	var (
		live = make(map[*ssa.BasicBlock]bool)
		walk func(*ssa.BasicBlock)
	)
	walk = func(block *ssa.BasicBlock) {
		if block == nil || live[block] {
			return
		}
		live[block] = true
		for _, succ := range block.Succs {
			walk(succ)
		}
	}
	if len(fn.Blocks) > 0 {
		walk(fn.Blocks[0])
	}
	walk(fn.Recover)
	return live
}

// read reports whether anything reads the parameter's initial value. A
// parameter whose address is used counts as used: the function may read it
// through the pointer.
func read(param *ssa.Parameter, reach map[*ssa.BasicBlock]bool) bool {
	refs := param.Referrers()
	if refs == nil {
		return false
	}
	for _, ref := range *refs {
		if reach[ref.Block()] {
			return true
		}
	}
	return false
}

// span is the source range of one node in a live control-flow block.
type span struct {
	from, to token.Pos
}

// livePositions holds the source ranges of the nodes in a function that a path
// through the body can reach. The second field is false when no control-flow
// graph exists.
type livePositions struct {
	spans []span
	ok    bool
}

// liveSpans returns the source ranges of the nodes in node's function that a
// path through the body can reach. A statement after a return or an expression
// statement that calls a function which does not return has no span.
func liveSpans(pass *analysis.Pass, node ast.Node) livePositions {
	cfgs, ok := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	if !ok {
		return livePositions{}
	}
	var graph *cfg.CFG
	switch fn := node.(type) {
	case *ast.FuncDecl:
		graph = cfgs.FuncDecl(fn)
	case *ast.FuncLit:
		graph = cfgs.FuncLit(fn)
	}
	if graph == nil {
		return livePositions{}
	}
	live := livePositions{spans: []span{}, ok: true}
	for _, block := range graph.Blocks {
		if !block.Live {
			continue
		}
		for _, n := range block.Nodes {
			live.spans = append(live.spans, span{n.Pos(), n.End()})
		}
	}
	return live
}

// reachable reports whether pos lies in one of live's spans. Without a
// control-flow graph, every position counts.
func reachable(live livePositions, pos token.Pos) bool {
	if !live.ok {
		return true
	}
	for _, s := range live.spans {
		if s.from <= pos && pos < s.to {
			return true
		}
	}
	return false
}

// checker is one function body and the positions it can reach.
type checker struct {
	pass  *analysis.Pass
	body  *ast.BlockStmt
	live  livePositions
	reach map[*ssa.BasicBlock]bool
	byObj map[types.Object]*ssa.Parameter
}

// assigned reports whether the body assigns to the parameter that name
// declares, in code the body can reach.
func (c *checker) assigned(name *ast.Ident) bool {
	var found bool
	ast.Inspect(c.body, func(n ast.Node) bool {
		if found {
			return false
		}
		var targets []ast.Expr
		switch n := n.(type) {
		case *ast.AssignStmt:
			targets = n.Lhs
		case *ast.RangeStmt:
			targets = []ast.Expr{n.Key, n.Value}
		default:
			return true
		}
		obj := c.pass.TypesInfo.Defs[name]
		for _, target := range targets {
			name, ok := ast.Unparen(target).(*ast.Ident)
			if !ok || c.pass.TypesInfo.ObjectOf(name) != obj {
				continue
			}
			if !reachable(c.live, target.Pos()) {
				continue
			}
			if c.readsIn(n, target, obj) {
				continue
			}
			found = true
			return false
		}
		return true
	})
	return found
}

// readsIn reports whether the assignment reads obj before or while it writes
// target. SSA removes a self-assignment such as x = x, so the assignment
// itself has to show the read.
func (c *checker) readsIn(node ast.Node, target ast.Expr, obj types.Object) bool {
	switch n := node.(type) {
	case *ast.AssignStmt:
		if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
			return true
		}
		for i, rhs := range n.Rhs {
			if len(n.Lhs) == len(n.Rhs) && blank(n.Lhs[i]) {
				continue
			}
			if c.mentions(rhs, obj) {
				return true
			}
		}
		for _, lhs := range n.Lhs {
			if lhs == target {
				continue
			}
			name, ok := ast.Unparen(lhs).(*ast.Ident)
			if ok && c.pass.TypesInfo.ObjectOf(name) == obj {
				continue // another write to the same parameter
			}
			if c.mentions(lhs, obj) {
				return true
			}
		}
	case *ast.RangeStmt:
		return c.mentions(n.X, obj)
	}
	return false
}

// blank reports whether expr is the blank identifier.
func blank(expr ast.Expr) bool {
	name, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && name.Name == "_"
}

// mentions reports whether node names obj.
func (c *checker) mentions(node ast.Node, obj types.Object) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		name, ok := n.(*ast.Ident)
		if ok && c.pass.TypesInfo.ObjectOf(name) == obj {
			found = true
			return false
		}
		return true
	})
	return found
}
