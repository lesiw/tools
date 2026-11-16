// Package errshadow provides an analysis.Analyzer that reports a declaration
// directly in an if body that shadows an error variable the condition tested.
//
// The declarations and scope section of the spec says that an identifier
// declared in a block may be redeclared in an inner block, and that the
// identifier of the inner declaration denotes the entity declared by the inner
// declaration for as long as it is in scope.
//
// The if body is an inner block, so a later use of err denotes the inner
// variable.
//
// The declaration has to be the only non-blank variable the statement
// declares. A declaration in a nested bare block, a plain else block, or a
// loop header is left alone. So is a declaration in a body whose condition
// requires the error to be nil through direct nil comparisons and the boolean
// operators, since the body then runs with a nil error.
//
// No fix is offered: an assignment changes which variable later statements
// use, and a rename needs a name.
//
// The rule is inspired by staticcheck's SA4006 family of forgotten-error
// checks. The shadow lookup follows the shadow analyzer in
// golang.org/x/tools.
package errshadow

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errshadow",
	Doc:  "reports a declaration in an if body that shadows the tested error",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		checkFile(pass, file)
	}
	return nil, nil
}

// checkFile reports each declaration in file that shadows an error variable
// tested by an if statement that contains the declaration.
func checkFile(pass *analysis.Pass, file *ast.File) {
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, n)
		for _, ident := range declared(pass, n) {
			check(pass, stack, n, ident)
		}
		return true
	})
}

// declared returns the identifiers n declares. A function's parameters and
// results are not declarations inside a block, so their field node is not
// handled here.
func declared(pass *analysis.Pass, n ast.Node) []*ast.Ident {
	switch n := n.(type) {
	case *ast.AssignStmt:
		if n.Tok == token.DEFINE {
			return defined(pass, idents(n.Lhs))
		}
	case *ast.GenDecl:
		if n.Tok != token.VAR {
			return nil
		}
		var out []*ast.Ident
		for _, spec := range n.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok {
				out = append(out, value.Names...)
			}
		}
		return defined(pass, out)
	}
	return nil
}

// defined returns the identifiers in ids that declare a new object.
func defined(pass *analysis.Pass, ids []*ast.Ident) (out []*ast.Ident) {
	for _, id := range ids {
		if pass.TypesInfo.Defs[id] != nil {
			out = append(out, id)
		}
	}
	return out
}

// idents returns the identifiers in exprs, ignoring nil expressions and
// anything that is not a plain identifier.
func idents(exprs []ast.Expr) []*ast.Ident {
	out := make([]*ast.Ident, 0, len(exprs))
	for _, expr := range exprs {
		if id, ok := expr.(*ast.Ident); ok {
			out = append(out, id)
		}
	}
	return out
}

// check reports ident when it shadows an error variable that the if condition
// of its block tests.
func check(pass *analysis.Pass, stack []ast.Node, n ast.Node, ident *ast.Ident) {
	if ident.Name == "_" || !onlyVar(pass, n) {
		return
	}
	obj, ok := pass.TypesInfo.Defs[ident].(*types.Var)
	if !ok || obj.Parent() == nil || obj.Parent().Parent() == nil {
		return
	}
	// The nearest outer declaration with this name is the one this
	// declaration shadows.
	_, shadowed := obj.Parent().Parent().LookupParent(obj.Name(), obj.Pos())
	outer, _ := shadowed.(*types.Var)
	if outer == nil || !isError(outer.Type()) {
		return
	}
	if !testedInBlock(pass, stack, n, outer) {
		return
	}
	pass.Reportf(ident.Pos(), "declaration shadows the error variable %q", ident.Name)
}

// onlyVar reports whether n declares one non-blank variable.
func onlyVar(pass *analysis.Pass, n ast.Node) bool {
	var count int
	for _, ident := range declared(pass, n) {
		id, ok := pass.TypesInfo.Defs[ident].(*types.Var)
		if ok && id.Name() != "_" {
			count++
		}
	}
	return count == 1
}

// testedInBlock reports whether n is declared directly in a block whose if
// condition tests obj and does not require it to be nil.
func testedInBlock(pass *analysis.Pass, stack []ast.Node, n ast.Node, obj *types.Var) bool {
	block := -1
	for i, node := range slices.Backward(stack) {
		if _, ok := node.(*ast.BlockStmt); ok {
			block = i
			break
		}
	}
	if block < 1 || block+1 >= len(stack) {
		return false
	}
	ifStmt, ok := stack[block-1].(*ast.IfStmt)
	if !ok || ifStmt.Body != stack[block] {
		return false
	}
	direct := stack[block+1]
	for {
		label, ok := direct.(*ast.LabeledStmt)
		if !ok {
			break
		}
		direct = label.Stmt
	}
	if direct != n {
		if _, ok := direct.(*ast.DeclStmt); !ok {
			return false
		}
		if _, ok := n.(*ast.GenDecl); !ok {
			return false
		}
	}
	if nilOnly(pass, ifStmt.Cond, obj) {
		return false
	}
	return uses(pass, ifStmt.Cond, obj)
}

// states is a set of nil states for obj.
type states uint8

const (
	nilState    states = 1 << iota // obj is nil
	nonNilState                    // obj is not nil
)

// nilOnly reports whether the truth of expr implies that obj is nil.
func nilOnly(pass *analysis.Pass, expr ast.Expr, obj *types.Var) bool {
	yes, _ := truth(pass, expr, obj)
	return yes&nonNilState == 0
}

// truth returns the nil states in which expr can be true, and the nil states
// in which it can be false.
func truth(pass *analysis.Pass, expr ast.Expr, obj *types.Var) (yes, no states) {
	expr = ast.Unparen(expr)
	if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.Value != nil {
		if constant.BoolVal(tv.Value) {
			return nilState | nonNilState, 0
		}
		return 0, nilState | nonNilState
	}
	switch expr := expr.(type) {
	case *ast.BinaryExpr:
		switch expr.Op {
		case token.EQL, token.NEQ:
			if nilComparison(pass, expr, obj) {
				if expr.Op == token.EQL {
					return nilState, nonNilState
				}
				return nonNilState, nilState
			}
		case token.LAND:
			var (
				xyes, xno = truth(pass, expr.X, obj)
				yyes, yno = truth(pass, expr.Y, obj)
			)
			return xyes & yyes, xno | yno
		case token.LOR:
			var (
				xyes, xno = truth(pass, expr.X, obj)
				yyes, yno = truth(pass, expr.Y, obj)
			)
			return xyes | yyes, xno & yno
		}
	case *ast.UnaryExpr:
		if expr.Op == token.NOT {
			yes, no := truth(pass, expr.X, obj)
			return no, yes
		}
	}
	return nilState | nonNilState, nilState | nonNilState
}

// nilComparison reports whether expr compares obj with nil.
func nilComparison(pass *analysis.Pass, expr *ast.BinaryExpr, obj *types.Var) bool {
	if isNil(pass, expr.X) && mentions(pass, expr.Y, obj) {
		return true
	}
	return isNil(pass, expr.Y) && mentions(pass, expr.X, obj)
}

// isNil reports whether expr is the predeclared nil.
func isNil(pass *analysis.Pass, expr ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[ast.Unparen(expr)]
	return ok && tv.IsNil()
}

// mentions reports whether expr is obj.
func mentions(pass *analysis.Pass, expr ast.Expr, obj *types.Var) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == obj
}

// uses reports whether node mentions obj.
func uses(pass *analysis.Pass, node ast.Node, obj *types.Var) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		id, ok := n.(*ast.Ident)
		if ok && pass.TypesInfo.Uses[id] == obj {
			found = true
		}
		return !found
	})
	return found
}

// isError reports whether t implements the predeclared error interface.
func isError(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	// The behavior of types.Implements is unspecified for an uninstantiated
	// generic type.
	if _, ok := t.(*types.TypeParam); ok {
		return false
	}
	typ := types.Universe.Lookup("error").Type()
	return types.Implements(t, typ.Underlying().(*types.Interface))
}
