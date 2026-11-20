// Package errorsastypeshadow provides an analysis.Analyzer that reports an
// errors.AsType call that tests a zero value from an earlier call in the same
// if/else chain.
//
// Vendored from gopls' errorsastypeshadow
// (gopls/internal/analysis/errorsastypeshadow, golang.org/x/tools/gopls
// v0.24.0), which is internal and cannot be imported. The rule is upstream's;
// the code is adapted to this repository's helpers.
//
// The errors package says of AsType (https://pkg.go.dev/errors#AsType):
// "Otherwise, it returns the zero value of E and false." An else branch runs
// only when the call above it failed. A later branch that hands the earlier
// call's variable to another call hands it a zero value:
//
//	if err, ok := errors.AsType[*alphaError](err); ok {
//		use(err)
//	} else if err, ok := errors.AsType[*betaError](err); ok {
//		use(err)
//	}
//
// The second call passes the variable the call above declared, so it tests a
// nil *alphaError whatever the original error was. Its answer says nothing
// about the original error.
//
// The analyzer reads the chain from the call that declares the variable to the
// call that passes it. Both ends are the same shape: the init statement
// declares the variable from the call, and the condition names the second
// result. A statement whose condition tests anything else is not a link in the
// chain. A use between the two calls that could write the variable
// suppresses the report: the variable may then hold a value the author wrote.
//
// Calls in the errors and errors_test packages are not reported, as in gopls:
// those packages know their own API, and a test writes chains that are wrong
// on purpose.
//
// Report only: the repair renames a variable, and the author has to choose the
// name.
package errorsastypeshadow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/typename"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "errorsastypeshadow",
	Doc:  "reports an errors.AsType call that tests a zero value from an earlier call in the same if/else chain",
	Run:  run,
}

// asTypeCall is an if statement whose init declares a value and an ok result
// from an errors.AsType call, and whose condition is that ok name.
type asTypeCall struct {
	stmt  *ast.IfStmt
	value *ast.Ident
	call  *ast.CallExpr
}

// facts holds what the rule reads about a package: the calls in AST order,
// each call also keyed by the variable it declares, the uses of each variable,
// and the parent of every node.
type facts struct {
	calls   []*asTypeCall
	byVar   map[types.Object]*asTypeCall
	uses    map[types.Object][]*ast.Ident
	parents map[ast.Node]ast.Node
}

func run(pass *analysis.Pass) (any, error) {
	switch pass.Pkg.Path() {
	case "errors", "errors_test":
		// The package tests its own API against incorrect programs.
		return nil, nil
	}
	f := collect(pass)
	for _, call := range f.calls {
		check(pass, call, f)
	}
	return nil, nil
}

// collect reads the calls, variable uses and parent links of the package.
func collect(pass *analysis.Pass) facts {
	f := facts{
		byVar:   make(map[types.Object]*asTypeCall),
		uses:    make(map[types.Object][]*ast.Ident),
		parents: make(map[ast.Node]ast.Node),
	}
	for id, obj := range pass.TypesInfo.Uses {
		if _, ok := obj.(*types.Var); ok {
			f.uses[obj] = append(f.uses[obj], id)
		}
	}
	for _, file := range pass.Files {
		maps.Copy(f.parents, astexpr.Parents(file))
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			call, ok := declare(pass, ifStmt)
			if !ok {
				return true
			}
			f.calls = append(f.calls, call)
			if obj := pass.TypesInfo.Defs[call.value]; obj != nil {
				f.byVar[obj] = call
			}
			return true
		})
	}
	return f
}

// check reports call when it passes a variable that an earlier call in the
// same chain declared.
func check(pass *analysis.Pass, call *asTypeCall, f facts) {
	if len(call.call.Args) != 1 {
		return
	}
	arg, ok := call.call.Args[0].(*ast.Ident)
	if !ok {
		return
	}
	obj := pass.TypesInfo.Uses[arg]
	from, ok := f.byVar[obj]
	if !ok {
		return
	}
	// The ok branch runs only when the call succeeded, so the variable holds
	// the value the call found rather than its zero value.
	if contains(from.stmt.Body, call.call) {
		return
	}
	chain{
		from:    from,
		to:      call,
		arg:     arg,
		uses:    f.uses[obj],
		parents: f.parents,
	}.walk(pass)
}

// chain is a walk from the statement that declares the variable to the call
// that passes it.
type chain struct {
	from    *asTypeCall
	to      *asTypeCall
	arg     *ast.Ident
	uses    []*ast.Ident
	parents map[ast.Node]ast.Node
}

// walk reports the call when no use of the variable between the two could
// write it.
func (c chain) walk(pass *analysis.Pass) {
	for cur := c.from.stmt; cur.Else != nil; {
		next, ok := cur.Else.(*ast.IfStmt)
		if ok {
			if next == c.to.stmt {
				c.report(pass)
				return
			}
			// One branch of the chain runs, so only the init statement and
			// the condition are evaluated before the choice is made.
			if c.writesBranch(next) {
				return
			}
			if contains(next.Body, c.to.call) {
				if !c.writes(next.Body) {
					c.report(pass)
				}
				return
			}
			cur = next
			continue
		}
		// The block ends the chain, so it runs when every condition above it
		// failed, and the call is inside it.
		if !c.writes(cur.Else.(*ast.BlockStmt)) {
			c.report(pass)
		}
		return
	}
}

// writesBranch reports whether a branch's condition, or the init statement
// before it, could write the variable.
func (c chain) writesBranch(ifStmt *ast.IfStmt) bool {
	if c.writes(ifStmt.Cond) {
		return true
	}
	return ifStmt.Init != nil && c.writes(ifStmt.Init)
}

// writes reports whether a use of the variable inside node could write it.
func (c chain) writes(node ast.Node) bool {
	for _, id := range c.uses {
		if contains(node, id) && couldWrite(c.parents, id) {
			return true
		}
	}
	return false
}

func (c chain) report(pass *analysis.Pass) {
	name := types.TypeString(pass.TypesInfo.TypeOf(c.arg), typename.Of(pass))
	pass.Report(analysis.Diagnostic{
		Pos:     c.arg.Pos(),
		End:     c.arg.End(),
		Message: fmt.Sprintf("%s is the zero value of %s from the earlier AsType call", c.arg.Name, name),
	})
}

// declare returns the call an if statement's init declares a value from, when
// the init is a short variable declaration of an errors.AsType call and the
// condition is its ok name.
func declare(pass *analysis.Pass, ifStmt *ast.IfStmt) (*asTypeCall, bool) {
	init, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE {
		return nil, false
	}
	if len(init.Lhs) != 2 || len(init.Rhs) != 1 {
		return nil, false
	}
	call, ok := init.Rhs[0].(*ast.CallExpr)
	if !ok || !isAsType(pass, call) {
		return nil, false
	}
	// A short declaration names its left side.
	held := init.Lhs[0].(*ast.Ident)
	result := init.Lhs[1].(*ast.Ident)
	cond, ok := ifStmt.Cond.(*ast.Ident)
	if !ok || cond.Name != result.Name {
		return nil, false
	}
	return &asTypeCall{stmt: ifStmt, value: held, call: call}, true
}

// isAsType reports whether call is errors.AsType, matched by object rather
// than by name.
func isAsType(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && fn.Pkg() != nil &&
		fn.Pkg().Path() == "errors" && fn.Name() == "AsType"
}

// couldWrite reports whether a use of a variable sits where it could be
// written. A use in an argument, on the right side of an assignment, or in a
// binary expression only reads the variable. A use reached through a selector,
// an index expression, a type assertion or parentheses takes the position of
// the expression it sits in.
func couldWrite(parents map[ast.Node]ast.Node, id *ast.Ident) bool {
	var node ast.Node = id
	for {
		switch p := parents[node].(type) {
		case *ast.ParenExpr:
			node = p
		case *ast.TypeAssertExpr, *ast.SelectorExpr:
			// A selector or assertion holds the variable as its operand.
			node = p
		case *ast.IndexExpr:
			// The variable can sit in the index, which this does not clear.
			if p.X != node {
				return true
			}
			node = p
		case *ast.CallExpr:
			for _, arg := range p.Args {
				if arg == node {
					return false
				}
			}
			return true
		case *ast.AssignStmt:
			for _, rhs := range p.Rhs {
				if rhs == node {
					return false
				}
			}
			return true
		case *ast.BinaryExpr:
			return false
		default:
			return true
		}
	}
}

// contains reports whether the span of inner lies inside the span of outer.
func contains(outer, inner ast.Node) bool {
	return outer.Pos() <= inner.Pos() && inner.End() <= outer.End()
}
