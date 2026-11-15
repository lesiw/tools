// Package uncondrecursion provides an analysis.Analyzer that reports a
// recursive call that runs whenever the function runs.
//
// A function that calls itself on every entry recurses without end:
//
//	func f() { f() }
//
// The goroutine's stack grows until it reaches a limit, and then the program
// crashes with "fatal error: stack overflow". runtime/debug.SetMaxStack is
// documented as "useful mainly for limiting the damage done by goroutines that
// enter an infinite recursion". A call in a go statement starts a goroutine on
// every entry instead, so the function returns and the program starts
// goroutines forever. A deferred call runs as the frame unwinds, so it
// recurses when it returns.
//
// Report only: ending the recursion means choosing what the function does
// instead.
//
// The rule is adapted from revive's unconditional-recursion.
package uncondrecursion

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "uncondrecursion",
	Doc:  "reports a recursive call that runs whenever the function runs",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
			if !ok {
				continue
			}
			c := &checker{pass: pass, fn: obj}
			c.list(fn.Body.List, false)
		}
	}
	return nil, nil
}

// checker walks one function body, reporting the calls to that function that
// run whenever it runs.
type checker struct {
	pass *analysis.Pass
	fn   *types.Func

	// stopped records that something already walked can leave the function, so
	// a later call does not run on every entry.
	stopped bool
}

// list walks statements that run one after another. loop says whether a break
// or a continue in them leaves the rest of the list.
func (c *checker) list(stmts []ast.Stmt, loop bool) {
	for _, stmt := range stmts {
		c.stmt(stmt, loop)
	}
}

// stmt walks one statement. The body of a branch or a select clause is
// conditional and is not walked, and neither is the body of a for that has a
// condition, or of a range; a break, a continue, or a goto that can leave is
// recorded instead.
func (c *checker) stmt(stmt ast.Stmt, loop bool) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		c.list(s.List, loop)
	case *ast.LabeledStmt:
		c.stmt(s.Stmt, loop)
	case *ast.IfStmt:
		// The init and the condition run on every entry; the arms do not.
		c.stmt(s.Init, loop)
		c.expr(s.Cond)
		c.stopped = c.stopped || c.exits(s.Body, true, loop) ||
			c.exits(s.Else, true, loop)
	case *ast.ForStmt:
		// The init and the condition run on every entry. A loop with no
		// condition runs its body on every entry as well.
		c.stmt(s.Init, loop)
		c.expr(s.Cond)
		if s.Cond == nil {
			c.list(s.Body.List, true)
			// The loop is assumed to run until it breaks, so the walk stops
			// at it.
			c.stopped = true
		} else {
			c.stopped = c.stopped || c.exits(s.Body, false, false)
		}
	case *ast.RangeStmt:
		// The range expression runs on every entry; the body runs once per
		// element.
		c.expr(s.X)
		c.stopped = c.stopped || c.exits(s.Body, false, false)
	case *ast.SwitchStmt:
		c.stmt(s.Init, loop)
		c.expr(s.Tag)
		c.stopped = c.stopped || c.exits(s.Body, false, loop)
	case *ast.TypeSwitchStmt:
		c.stmt(s.Init, loop)
		c.expr(s.Assign)
		c.stopped = c.stopped || c.exits(s.Body, false, loop)
	case *ast.SelectStmt:
		c.comms(s.Body)
		// An empty select blocks forever.
		c.stopped = c.stopped || len(s.Body.List) == 0 ||
			c.exits(s.Body, false, loop)
	case *ast.BranchStmt:
		c.branch(s, loop)
	case *ast.GoStmt:
		// The callee starts a goroutine, and everything that produces it runs
		// here.
		c.expr(s.Call)
	case *ast.ReturnStmt:
		for _, expr := range s.Results {
			c.expr(expr)
		}
		c.stopped = true
	default:
		c.expr(stmt)
		c.stopped = c.stopped || c.leaves(stmt)
	}
}

// leaves reports whether stmt is a call that ends the goroutine, so nothing
// after it in the list runs.
func (c *checker) leaves(stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := ast.Unparen(expr.X).(*ast.CallExpr)
	return ok && c.ends(call)
}

// branch records whether a branch leaves the rest of the statement list. A
// goto leaves it, since the walk does not track where the label is.
func (c *checker) branch(stmt *ast.BranchStmt, loop bool) {
	switch stmt.Tok {
	case token.GOTO:
		c.stopped = true
	case token.BREAK, token.CONTINUE:
		// A labeled one names a target of its own, and a bare one leaves the
		// rest of the loop body.
		if loop || stmt.Label != nil {
			c.stopped = true
		}
	}
}

// comms reports every self-call in the channel and send expressions of a
// select, which are evaluated on entry.
func (c *checker) comms(body *ast.BlockStmt) {
	for _, stmt := range body.List {
		if clause, ok := stmt.(*ast.CommClause); ok {
			c.expr(clause.Comm)
		}
	}
}

// expr reports every self-call in node, leaving the body of a function literal
// alone: a call there runs when the literal runs. The right operand of a && or
// a || runs only when the left one does not settle the result, so only the
// left one is walked.
func (c *checker) expr(node ast.Node) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				c.expr(n.X)
				return false
			}
		case *ast.CallExpr:
			c.report(n)
		}
		return true
	})
}

// report reports a self-call, unless something already walked can leave the
// function and end the recursion.
func (c *checker) report(call *ast.CallExpr) {
	if c.stopped || typeutil.StaticCallee(c.pass.TypesInfo, call) != c.fn {
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "unconditional recursive call",
	})
}

// exits reports whether node holds a branch that leaves the function or the
// rest of the statement list being walked. brk says a bare break targets a
// statement outside the list, and cont says a bare continue targets a loop
// outside it.
func (c *checker) exits(node ast.Node, brk, cont bool) bool {
	if node == nil {
		return false
	}
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			// A return there leaves the literal, not this function.
			return false
		case *ast.DeferStmt:
			// Nothing under a defer runs now.
			return false
		case *ast.ReturnStmt:
			found = true
		case *ast.BranchStmt:
			switch n.Tok {
			case token.GOTO:
				found = true
			case token.BREAK:
				found = n.Label != nil || brk
			case token.CONTINUE:
				found = n.Label != nil || cont
			}
		case *ast.CallExpr:
			found = c.ends(n)
		case *ast.ForStmt, *ast.RangeStmt:
			// A break or a continue there belongs to that loop, so only what
			// outlives it is scanned.
			found = c.exits(braced(n), false, false)
			return false
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			// A break there belongs to that statement. A continue reaches
			// past it to the loop the walk is inside.
			found = c.exits(braced(n), false, cont)
			return false
		}
		return !found
	})
	return found
}

// braced returns the body of a loop, a switch, or a select, whose break
// belongs to it.
func braced(node ast.Node) *ast.BlockStmt {
	switch n := node.(type) {
	case *ast.ForStmt:
		return n.Body
	case *ast.RangeStmt:
		return n.Body
	case *ast.SwitchStmt:
		return n.Body
	case *ast.TypeSwitchStmt:
		return n.Body
	case *ast.SelectStmt:
		return n.Body
	}
	return nil
}

// ends reports whether call ends the goroutine running the function.
func (c *checker) ends(call *ast.CallExpr) bool {
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if c.pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic") {
			return true
		}
	}
	fn := callee(c.pass.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "os":
		return fn.Name() == "Exit"
	case "runtime":
		return fn.Name() == "Goexit"
	case "log":
		switch fn.Name() {
		case "Fatal", "Fatalf", "Fatalln", "Panic", "Panicf", "Panicln":
			return true
		}
	case "testing":
		switch fn.Name() {
		case "Fatal", "Fatalf", "FailNow":
			return true
		case "Skip", "Skipf", "SkipNow":
			return true
		}
	}
	return false
}

// callee returns the function that call names, including one reached through
// an interface, which typeutil.StaticCallee drops.
func callee(info *types.Info, call *ast.CallExpr) *types.Func {
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	return fn
}
