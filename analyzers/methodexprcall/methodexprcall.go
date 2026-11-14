// Package methodexprcall provides an analysis.Analyzer that reports a method
// expression called with its receiver.
//
// T.Method(x) writes the receiver twice, once as the type it is declared on
// and once as the value it is called with. The spec's method expressions
// section calls t.Mv(7) and T.Mv(t, 7) equivalent, so this rule reports the
// form that names both.
//
// The fix writes the method call in place. Where a comment sits in the text
// the fix would delete, the call is reported without a fix.
//
// The rule is adapted from go-critic's methodExprCall, which reports only.
package methodexprcall

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "methodexprcall",
	Doc:  "reports a method expression called with its receiver",
	Run:  run,
}

// span is a range of source text.
type span struct {
	lo, hi token.Pos
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if call, ok := n.(*ast.CallExpr); ok {
				report(pass, file, stack[:len(stack)-1], call)
			}
			return true
		})
	}
	return nil, nil
}

// report writes the method call as two edits around the receiver, so the
// receiver's own text stays where it is and a call inside it keeps its edits.
func report(pass *analysis.Pass, file *ast.File, stack []ast.Node, call *ast.CallExpr) {
	selection := methodExpr(pass, call)
	if selection == nil {
		return
	}
	name := selection.Obj().Name()
	recv := ast.Unparen(call.Args[0])
	if !resolvesTo(pass, recv, selection, name) {
		return
	}
	recv = strip(pass, recv, selection, name)
	end := call.Rparen
	if len(call.Args) > 1 {
		end = call.Args[1].Pos()
	}
	d := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "method expression called with its receiver; use a method call",
	}
	callee := span{call.Pos(), recv.Pos()}
	comma := span{recv.End(), end}
	if dropsComment(file, callee, comma) {
		pass.Report(d)
		return
	}
	before, after := "", "."+name+"("
	if needsParens(pass, recv, stack) {
		before, after = "(", ")."+name+"("
	}
	d.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Write the method call",
		TextEdits: []analysis.TextEdit{
			{Pos: callee.lo, End: callee.hi, NewText: []byte(before)},
			{Pos: comma.lo, End: comma.hi, NewText: []byte(after)},
		},
	}}
	pass.Report(d)
}

// methodExpr returns the method expression a call calls, if it calls one. A
// selector on a value is a method value, which binds its receiver where it is
// written.
func methodExpr(pass *analysis.Pass, call *ast.CallExpr) *types.Selection {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodExpr {
		return nil
	}
	return selection
}

// strip returns the expression a method call names on its own, dropping the
// address or the dereference the method expression's form forced.
func strip(pass *analysis.Pass, e ast.Expr, selection *types.Selection, name string) ast.Expr {
	var inner ast.Expr
	switch e := e.(type) {
	case *ast.UnaryExpr:
		if e.Op != token.AND {
			return e
		}
		inner = e.X
	case *ast.StarExpr:
		inner = e.X
	default:
		return e
	}
	operand := ast.Unparen(inner)
	if resolvesTo(pass, operand, selection, name) {
		return operand
	}
	return e
}

// resolvesTo reports whether a method call on e, under the name the method
// expression gives, reaches the method that expression names.
//
// An untyped value has no method set of its own, and the call converts it; a
// tuple from a multiple-value call has none either.
//
// Otherwise the argument's own type has to hold the method the expression
// names: an unnamed type declares none, and a promoted field or another type's
// method under that name is not the one the expression names.
func resolvesTo(pass *analysis.Pass, e ast.Expr, selection *types.Selection, name string) bool {
	tv := pass.TypesInfo.Types[e]
	if untyped(pass, e) {
		return false
	}
	addr := tv.Addressable()
	obj, _, _ := types.LookupFieldOrMethod(tv.Type, addr, pass.Pkg, name)
	if obj == nil {
		return false
	}
	// An interface receiver dispatches on the dynamic type, which a method
	// call on the value does too. Everywhere else the call has to name the
	// same method.
	if _, ok := selection.Recv().Underlying().(*types.Interface); ok {
		return true
	}
	return obj == selection.Obj()
}

// untyped reports whether e's value is untyped, which the call converts to
// the receiver's type. go/types records the converted type for the argument,
// so the text of the expression is what says whether it ever had one. An
// untyped nil is left to the lookup, which finds no method to call.
func untyped(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return untypedConst(pass.TypesInfo.ObjectOf(e))
	case *ast.SelectorExpr:
		return untypedConst(pass.TypesInfo.ObjectOf(e.Sel))
	case *ast.CallExpr:
		if pass.TypesInfo.Types[e].Value == nil {
			return false // a call yields a typed value
		}
		// A conversion writes a type. A builtin call leaves a constant's type
		// to the record, which holds only the converted one, so min(1, 2)
		// cannot be told from min(three, four): both are left alone.
		return !pass.TypesInfo.Types[e.Fun].IsType()
	case *ast.BinaryExpr:
		// A comparison yields an untyped bool whatever its operands. Every
		// other operator keeps the type its operands decide, except a shift,
		// which takes the type of its left operand alone: the call gives
		// count.twice(1 << n) a receiver type, and (1 << n).twice() has none.
		switch e.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR,
			token.GEQ:
			return true
		case token.SHL, token.SHR:
			return untyped(pass, e.X)
		}
		return untyped(pass, e.X) && untyped(pass, e.Y)
	case *ast.UnaryExpr:
		return untyped(pass, e.X)
	}
	// A literal has no type of its own; another expression has one.
	return pass.TypesInfo.Types[e].Value != nil
}

// untypedConst reports whether obj is a constant declared without a type.
func untypedConst(obj types.Object) bool {
	c, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	basic, ok := c.Type().Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsUntyped != 0
}

// needsParens reports whether the method call needs parentheses around e, the
// expression it takes as its receiver.
func needsParens(pass *analysis.Pass, e ast.Expr, stack []ast.Node) bool {
	switch e.(type) {
	case *ast.StarExpr, *ast.UnaryExpr, *ast.BinaryExpr:
		return true
	}
	// A brace at the head of a control statement's header reads as the start
	// of the block unless a bracket already separates the two.
	if _, ok := head(e).(*ast.CompositeLit); !ok {
		return false
	}
	return bare(pass, stack, e)
}

// head returns the expression that begins with e's first token.
func head(e ast.Expr) ast.Expr {
	for {
		switch n := e.(type) {
		case *ast.SelectorExpr:
			e = n.X
		case *ast.IndexExpr:
			e = n.X
		case *ast.IndexListExpr:
			e = n.X
		case *ast.CallExpr:
			e = n.Fun
		case *ast.SliceExpr:
			e = n.X
		case *ast.TypeAssertExpr:
			e = n.X
		default:
			return e
		}
	}
}

// bare reports whether stack puts e in a control statement's header with no
// parenthesis around it, where a composite literal's brace would read as the
// start of the block. A block ends the walk: a body of its own, as a closure's
// or a switch's, already separates the literal from the keyword.
func bare(pass *analysis.Pass, stack []ast.Node, e ast.Expr) bool {
	for _, n := range slices.Backward(stack) {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt:
			return true
		case *ast.BlockStmt:
			return false
		}
		if holds(pass, n, e.Pos(), e.End()) {
			return false
		}
	}
	return false
}

// holds reports whether a bracket n encloses the text from lo to hi, and stays
// around it when the text moves.
func holds(pass *analysis.Pass, n ast.Node, lo, hi token.Pos) bool {
	switch n := n.(type) {
	case *ast.ParenExpr:
		return n.Lparen < lo && hi <= n.Rparen
	case *ast.CallExpr:
		if hi > n.Rparen || lo <= n.Lparen {
			return false
		}
		// A call this rule rewrites moves its receiver out of its own
		// parentheses, but the arguments after the receiver stay in them.
		if methodExpr(pass, n) != nil {
			return len(n.Args) > 1 && n.Args[1].Pos() <= lo
		}
		return true
	case *ast.IndexExpr:
		return n.Lbrack < lo && hi <= n.Rbrack
	case *ast.IndexListExpr:
		return n.Lbrack < lo && hi <= n.Rbrack
	case *ast.CompositeLit:
		return n.Lbrace < lo && hi <= n.Rbrace
	case *ast.SliceExpr:
		return n.Lbrack < lo && hi <= n.Rbrack
	}
	return false
}

// dropsComment reports whether a comment falls in a span the fix deletes.
func dropsComment(file *ast.File, spans ...span) bool {
	for _, group := range file.Comments {
		for _, s := range spans {
			if group.Pos() < s.hi && group.End() > s.lo {
				return true
			}
		}
	}
	return false
}
