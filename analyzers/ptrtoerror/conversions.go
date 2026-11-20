package ptrtoerror

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// A conversion is one place a value is assigned to a type, or a type is tested
// as one. A test names the tested type in from and the interface it is read
// from in to, and an instantiation names the type argument in from and its
// constraint in to.
type conversion struct {
	to, from types.Type
	expr     ast.Expr
}

// conversions returns the package's conversions in source order.
func conversions(pass *analysis.Pass) []conversion {
	c := &collector{info: pass.TypesInfo}
	for _, file := range pass.Files {
		c.funcs = c.funcs[:0]
		ast.Inspect(file, c.visit)
	}
	c.instances()
	slices.SortStableFunc(c.out, func(a, b conversion) int {
		return cmp.Compare(a.expr.Pos(), b.expr.Pos())
	})
	return c.out
}

// A collector walks a package looking for conversions.
type collector struct {
	info  *types.Info
	out   []conversion
	funcs []frame // the enclosing functions, innermost last
}

// A frame is one enclosing function, for the signature a return statement
// assigns to.
type frame struct {
	end token.Pos
	sig *types.Signature
}

// add records a conversion, and drops one whose destination or operand has no
// type of its own, as the blank identifier has none.
func (c *collector) add(to, from types.Type, expr ast.Expr) {
	if to == nil || from == nil {
		return
	}
	c.out = append(c.out, conversion{to: to, from: from, expr: expr})
}

func (c *collector) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.FuncDecl:
		var t types.Type
		if obj := c.info.Defs[n.Name]; obj != nil {
			t = obj.Type()
		}
		c.funcs = append(c.funcs, frame{n.End(), signature(t)})
	case *ast.FuncLit:
		c.funcs = append(c.funcs, frame{n.End(), signature(c.info.TypeOf(n))})
	case *ast.AssignStmt:
		c.assign(n)
	case *ast.ValueSpec:
		c.valueSpec(n)
	case *ast.CallExpr:
		c.call(n)
	case *ast.ReturnStmt:
		c.returnStmt(n)
	case *ast.CompositeLit:
		c.compositeLit(n)
	case *ast.SendStmt:
		c.send(n)
	case *ast.TypeAssertExpr:
		c.typeAssert(n)
	case *ast.TypeSwitchStmt:
		c.typeSwitch(n)
	}
	return true
}

// enclosing returns the signature of the innermost function that holds pos.
func (c *collector) enclosing(pos token.Pos) *types.Signature {
	for len(c.funcs) > 0 && c.funcs[len(c.funcs)-1].end <= pos {
		c.funcs = c.funcs[:len(c.funcs)-1]
	}
	if len(c.funcs) == 0 {
		return nil
	}
	return c.funcs[len(c.funcs)-1].sig
}

// signature returns a function's type as a signature, and nil for a function
// with no type of its own.
func signature(t types.Type) *types.Signature {
	if t == nil {
		return nil
	}
	sig, _ := t.Underlying().(*types.Signature)
	return sig
}

func (c *collector) assign(n *ast.AssignStmt) {
	switch {
	case len(n.Lhs) == len(n.Rhs):
		for i, lhs := range n.Lhs {
			c.add(c.info.TypeOf(lhs), c.info.TypeOf(n.Rhs[i]), n.Rhs[i])
		}
	case len(n.Lhs) > 1 && len(n.Rhs) == 1:
		tuple, ok := c.info.TypeOf(n.Rhs[0]).(*types.Tuple)
		if !ok || tuple.Len() != len(n.Lhs) {
			return
		}
		for i, lhs := range n.Lhs {
			c.add(c.info.TypeOf(lhs), tuple.At(i).Type(), n.Rhs[0])
		}
	}
}

func (c *collector) valueSpec(n *ast.ValueSpec) {
	if len(n.Values) == 0 {
		return
	}
	switch {
	case len(n.Names) == len(n.Values):
		for i := range n.Names {
			if obj := c.info.ObjectOf(n.Names[i]); obj != nil {
				c.add(obj.Type(), c.info.TypeOf(n.Values[i]), n.Values[i])
			}
		}
	case len(n.Names) > 1 && len(n.Values) == 1:
		tuple, ok := c.info.TypeOf(n.Values[0]).(*types.Tuple)
		if !ok || tuple.Len() != len(n.Names) {
			return
		}
		for i := range n.Names {
			if obj := c.info.ObjectOf(n.Names[i]); obj != nil {
				c.add(obj.Type(), tuple.At(i).Type(), n.Values[0])
			}
		}
	}
}

func (c *collector) call(n *ast.CallExpr) {
	tv, ok := c.info.Types[n.Fun]
	if !ok || tv.Type == nil {
		return
	}
	if tv.IsType() {
		if len(n.Args) > 0 {
			c.add(tv.Type, c.info.TypeOf(n.Args[0]), n.Args[0])
		}
		return
	}
	sig, ok := tv.Type.Underlying().(*types.Signature)
	if !ok || len(n.Args) == 0 || sig.Params().Len() == 0 {
		return
	}
	if len(n.Args) == 1 && sig.Params().Len() > 1 {
		c.tuple(n.Args[0], sig.Params())
		return
	}
	c.params(n.Args, n.Ellipsis.IsValid(), sig)
}

// params records the conversions of the arguments of a call to its
// parameters.
func (c *collector) params(args []ast.Expr, ellipsis bool, sig *types.Signature) {
	var (
		params = sig.Params()
		last   = params.Len() - 1
	)
	for i, arg := range args {
		if !sig.Variadic() || i < last {
			if i < params.Len() {
				c.add(params.At(i).Type(), c.info.TypeOf(arg), arg)
			}
			continue
		}
		c.variadic(arg, params.At(last).Type(), ellipsis)
	}
}

// variadic records an argument that goes to a variadic parameter. With an
// ellipsis the argument is the slice itself, and without one it is an element.
func (c *collector) variadic(arg ast.Expr, param types.Type, ellipsis bool) {
	slice, ok := param.(*types.Slice)
	if !ok {
		return
	}
	var to types.Type = slice
	if !ellipsis {
		to = slice.Elem()
	}
	c.add(to, c.info.TypeOf(arg), arg)
}

// tuple records a value that stands for a list of destinations, as one result
// of a call stands for several parameters and several results.
func (c *collector) tuple(expr ast.Expr, to *types.Tuple) {
	from, ok := c.info.TypeOf(expr).(*types.Tuple)
	if !ok {
		return
	}
	for i := 0; i < to.Len() && i < from.Len(); i++ {
		c.add(to.At(i).Type(), from.At(i).Type(), expr)
	}
}

func (c *collector) returnStmt(n *ast.ReturnStmt) {
	sig := c.enclosing(n.Pos())
	if sig == nil {
		return
	}
	if len(n.Results) == 1 && sig.Results().Len() > 1 {
		c.tuple(n.Results[0], sig.Results())
		return
	}
	if len(n.Results) != sig.Results().Len() {
		return
	}
	for i, res := range n.Results {
		c.add(sig.Results().At(i).Type(), c.info.TypeOf(res), res)
	}
}

func (c *collector) compositeLit(n *ast.CompositeLit) {
	t := c.info.TypeOf(n)
	if t == nil {
		return
	}
	switch t := t.Underlying().(type) {
	case *types.Slice:
		c.elements(n, t.Elem())
	case *types.Array:
		c.elements(n, t.Elem())
	case *types.Map:
		for _, elt := range n.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			c.add(t.Key(), c.info.TypeOf(kv.Key), kv.Key)
			c.add(t.Elem(), c.info.TypeOf(kv.Value), kv.Value)
		}
	case *types.Struct:
		for i, elt := range n.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				if i < t.NumFields() {
					c.add(t.Field(i).Type(), c.info.TypeOf(elt), elt)
				}
				continue
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			if field := c.info.Uses[id]; field != nil {
				c.add(field.Type(), c.info.TypeOf(kv.Value), kv.Value)
			}
		}
	}
}

// elements records the conversions of the elements of a list literal.
func (c *collector) elements(n *ast.CompositeLit, elem types.Type) {
	for _, elt := range n.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			elt = kv.Value
		}
		c.add(elem, c.info.TypeOf(elt), elt)
	}
}

func (c *collector) send(n *ast.SendStmt) {
	t := c.info.TypeOf(n.Chan)
	if t == nil {
		return
	}
	ch, ok := t.Underlying().(*types.Chan)
	if !ok {
		return
	}
	c.add(ch.Elem(), c.info.TypeOf(n.Value), n.Value)
}

// typeAssert records a tested type as a conversion to the interface tested. A
// type switch's assertion has no type of its own, so its cases are read by
// typeSwitch.
func (c *collector) typeAssert(n *ast.TypeAssertExpr) {
	if n.Type == nil {
		return
	}
	c.add(c.info.TypeOf(n.X), c.info.TypeOf(n.Type), n.Type)
}

func (c *collector) typeSwitch(n *ast.TypeSwitchStmt) {
	var assert *ast.TypeAssertExpr
	switch assign := n.Assign.(type) {
	case *ast.ExprStmt:
		assert, _ = assign.X.(*ast.TypeAssertExpr)
	case *ast.AssignStmt:
		if len(assign.Rhs) > 0 {
			assert, _ = assign.Rhs[0].(*ast.TypeAssertExpr)
		}
	}
	if assert == nil {
		return
	}
	for _, clause := range n.Body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, typ := range cc.List {
			c.add(c.info.TypeOf(assert.X), c.info.TypeOf(typ), typ)
		}
	}
}

// instances records the arguments a type parameter is instantiated with, as
// errors.AsType[E] gives E.
func (c *collector) instances() {
	for id := range c.info.Instances {
		obj := c.info.ObjectOf(id)
		if obj == nil {
			continue
		}
		hasTypeParams, ok := obj.Type().(interface {
			TypeParams() *types.TypeParamList
		})
		if !ok {
			continue
		}
		params := hasTypeParams.TypeParams()
		args := c.info.Instances[id].TypeArgs
		for i := 0; i < params.Len() && i < args.Len(); i++ {
			c.add(params.At(i).Constraint(), args.At(i), id)
		}
	}
}
