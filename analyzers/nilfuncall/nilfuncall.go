// Package nilfuncall provides an analysis.Analyzer that reports a call of a
// function value that the same block set to nil and did not write again.
//
// The specification's Calls section says "Calling a nil function value causes
// a run-time panic" (https://go.dev/ref/spec#Calls).
//
// Report only: dropping the assignment makes the call run the function the
// variable held before, and dropping the call drops the work.
//
// The suite's nilness pass reports most of the same calls from SSA. It does
// not read a package-level literal, and loses the value in a literal that
// captures the variable.
//
// The README catalogs every check and exemption.
package nilfuncall

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilfuncall",
	Doc:  "reports a call of a function value that the same block set to nil",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					check(pass, n.Recv, n.Type, n.Body, nil)
				}
				return false
			case *ast.FuncLit:
				// The literal initializes a package-level variable. Every
				// other literal is inside a function body, which check walks.
				check(pass, nil, n.Type, n.Body, nil)
				return false
			}
			return true
		})
	}
	return nil, nil
}

// check examines one function body. outer is the skip set of the function
// that holds it.
func check(pass *analysis.Pass, recv *ast.FieldList, ft *ast.FuncType, body *ast.BlockStmt, outer vars) {
	var (
		base, writes = skipped(pass, recv, ft, body, outer)
		skip         = copyOf(base)
	)
	for _, own := range writes {
		skip.add(own)
	}
	for _, lit := range literals(body) {
		// A literal reads its own statements in order, so only a write from
		// another literal can run before its call.
		inner := copyOf(base)
		for other, own := range writes {
			if other != lit {
				inner.add(own)
			}
		}
		check(pass, nil, lit.Type, lit.Body, inner)
	}
	if jumps(body) {
		return
	}
	c := checker{pass: pass, skip: skip}
	c.list(vars{}, body.List)
}

// checker reports the calls in one function.
type checker struct {
	pass *analysis.Pass
	skip vars
}

// vars is a set of variables. A block's nil set and a function's skip set are
// both one of these.
type vars map[types.Object]struct{}

// copyOf returns a copy of set. A caller passes nil, so the result is always
// a writable map.
func copyOf(set vars) vars {
	out := make(vars, len(set))
	for obj := range set {
		out[obj] = struct{}{}
	}
	return out
}

// add puts the variables of other in v.
func (v vars) add(other vars) {
	for obj := range other {
		v[obj] = struct{}{}
	}
}

// list walks one block in statement order.
func (c checker) list(known vars, stmts []ast.Stmt) {
	for _, stmt := range stmts {
		c.statement(known, stmt)
	}
}

// statement walks one statement of a block.
func (c checker) statement(known vars, stmt ast.Stmt) {
	// A write anywhere in the statement clears the variable, including in a
	// nested block, since a later statement's call may run after it.
	for _, obj := range assigned(c.pass, stmt) {
		delete(known, obj)
	}
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		c.list(vars{}, s.List)
	case *ast.CaseClause:
		c.list(vars{}, s.Body)
	case *ast.CommClause:
		c.list(vars{}, s.Body)
	case *ast.IfStmt:
		c.list(vars{}, s.Body.List)
		if s.Else != nil {
			c.statement(vars{}, s.Else)
		}
	case *ast.ForStmt:
		c.list(vars{}, s.Body.List)
	case *ast.RangeStmt:
		c.list(vars{}, s.Body.List)
	case *ast.SwitchStmt:
		c.list(vars{}, s.Body.List)
	case *ast.TypeSwitchStmt:
		c.list(vars{}, s.Body.List)
	case *ast.SelectStmt:
		c.list(vars{}, s.Body.List)
	case *ast.LabeledStmt:
		// A label does not open a block.
		c.statement(known, s.Stmt)
	default:
		c.calls(known, s)
		c.set(known, s)
	}
}

// calls reports the calls in one statement or expression, and not in a
// function literal, whose body is a function of its own.
func (c checker) calls(known vars, node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			c.call(known, n)
		case *ast.FuncLit:
			return false
		}
		return true
	})
}

// call reports a call through a variable known to be nil.
func (c checker) call(known vars, call *ast.CallExpr) {
	obj := c.variable(call.Fun)
	if obj == nil {
		return
	}
	if _, ok := known[obj]; !ok {
		return
	}
	if _, ok := c.skip[obj]; ok {
		return
	}
	c.pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "function value is nil; the call panics",
	})
}

// set records the variables a statement sets to nil, for the calls that follow
// it in the same block.
func (c checker) set(known vars, stmt ast.Stmt) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return
	}
	// A right side that assigns a value this walk does not read leaves the
	// variables already cleared above.
	if len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	// The last pair to write a variable gives it its value, so a later pair
	// overwrites what an earlier one records.
	for i, lhs := range assign.Lhs {
		obj := c.variable(lhs)
		if obj == nil {
			continue
		}
		if c.isNil(assign.Rhs[i]) && c.local(obj) {
			known[obj] = struct{}{}
		} else {
			delete(known, obj)
		}
	}
}

// variable returns the variable e names. The left side of a := is a
// definition, so assigned reads ObjectOf where this reads a use.
func (c checker) variable(e ast.Expr) *types.Var {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	obj, _ := c.pass.TypesInfo.Uses[id].(*types.Var)
	return obj
}

// isNil reports whether e is the predeclared nil.
func (c checker) isNil(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && c.pass.TypesInfo.Uses[id] == types.Universe.Lookup("nil")
}

// local reports whether obj is a local variable of this package.
func (c checker) local(obj *types.Var) bool {
	return obj.Pkg() == c.pass.Pkg && obj.Parent() != c.pass.Pkg.Scope()
}

// skipped returns the variables the function leaves alone apart from its
// literals, and the variables each literal in the body writes.
func skipped(pass *analysis.Pass, recv *ast.FieldList, ft *ast.FuncType, body *ast.BlockStmt, outer vars) (vars, map[*ast.FuncLit]vars) {
	base := copyOf(outer)
	for _, list := range []*ast.FieldList{recv, ft.Params, ft.Results} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				if obj := pass.TypesInfo.ObjectOf(name); obj != nil {
					base[obj] = struct{}{}
				}
			}
		}
	}
	writes := make(map[*ast.FuncLit]vars)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				if id, ok := ast.Unparen(n.X).(*ast.Ident); ok {
					if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
						base[obj] = struct{}{}
					}
				}
			}
		case *ast.FuncLit:
			own := vars{}
			for _, obj := range assigned(pass, n.Body) {
				// A variable the literal declares belongs to the literal,
				// which check reads on its own.
				if declaredOutside(obj, n.Body) {
					own[obj] = struct{}{}
				}
			}
			writes[n] = own
		}
		return true
	})
	return base, writes
}

// declaredOutside reports whether obj is declared outside body.
func declaredOutside(obj types.Object, body *ast.BlockStmt) bool {
	pos := obj.Pos()
	return pos < body.Lbrace || pos > body.Rbrace
}

// literals returns the function literals in body, stopping at each one.
func literals(body *ast.BlockStmt) (out []*ast.FuncLit) {
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			out = append(out, lit)
			return false
		}
		return true
	})
	return out
}

// jumps reports whether body holds a goto outside a function literal.
func jumps(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			found = found || n.Tok == token.GOTO
		}
		return true
	})
	return found
}

// assigned returns the variables stmt writes, including in a nested block. A
// function literal's body is a function of its own, so this does not walk it.
func assigned(pass *analysis.Pass, stmt ast.Stmt) (out []types.Object) {
	add := func(e ast.Expr) {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok || id.Name == "_" {
			return
		}
		if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
			out = append(out, obj)
		}
	}
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				add(lhs)
			}
		case *ast.RangeStmt:
			add(n.Key)
			add(n.Value)
		case *ast.FuncLit:
			return false
		}
		return true
	})
	return out
}
