// Package nilchecklen provides an analysis.Analyzer that reports a nil check
// that a len comparison makes redundant.
//
// The specification's "Length and capacity" section says the length of a nil
// slice, map or channel is 0. A nil check joined to a comparison of len with a
// constant is redundant when the comparison is false at a length of zero under
// &&, or true at a length of zero under ||. The nil check tests != nil under
// && and == nil under ||. The constant may be written on either side, and
// either operand of the pair may be the nil check. The value has to contain no
// call and no channel receive, so that both reads see the same value. A
// pointer to an array does not qualify, since len of a nil one is the array
// length, and a type parameter qualifies when its type set holds nothing but
// those.
//
// The nil check and the comparison have to be adjacent operands of one && or
// ||. An operand between them ends the pair: the rule reads neighbors only, so
// a nil check that another operand separates from its comparison is left
// alone.
//
// The fix deletes the nil check, the && or || that joins it to the comparison,
// and the parentheses around the comparison. A comment inside the deleted text
// withholds the fix, because the comment belongs to the author.
//
// The idea comes from staticcheck's S1009, which reports the pair only when
// the nil check is the left operand of the && or || and the len call is the
// left operand of its comparison.
package nilchecklen

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilchecklen",
	Doc:  "reports a nil check that a len comparison makes redundant",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		nested := make(map[*ast.BinaryExpr]bool)
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || (bin.Op != token.LAND && bin.Op != token.LOR) {
				return true
			}
			if nested[bin] {
				return true
			}
			operands := flatten(bin, nested)
			// A nil check between two qualifying comparisons is reported
			// once.
			for i, e := range operands {
				if i > 0 && report(pass, file, bin.Op, e, operands[i-1]) {
					continue
				}
				if i+1 < len(operands) {
					report(pass, file, bin.Op, e, operands[i+1])
				}
			}
			return true
		})
	}
	return nil, nil
}

// flatten returns the operands bin joins with its operator, in order, and
// marks every nested chain of the same operator so that it is not visited
// again.
func flatten(bin *ast.BinaryExpr, nested map[*ast.BinaryExpr]bool) []ast.Expr {
	out := make([]ast.Expr, 0, 2)
	for _, e := range [2]ast.Expr{bin.X, bin.Y} {
		next, ok := e.(*ast.BinaryExpr)
		if !ok || next.Op != bin.Op {
			out = append(out, e)
			continue
		}
		nested[next] = true
		out = append(out, flatten(next, nested)...)
	}
	return out
}

// report emits a diagnostic when other, a len comparison on the value nilCheck
// tests, makes the nil check redundant. It reports whether it did.
func report(pass *analysis.Pass, file *ast.File, op token.Token, nilCheck, other ast.Expr) bool {
	x, ok := nilOperand(pass, op, nilCheck)
	if !ok || effect(pass, x) {
		return false
	}
	msg := message(pass, x)
	if msg == "" {
		return false
	}
	cmp, ok := ast.Unparen(other).(*ast.BinaryExpr)
	if !ok || !redundant(pass, op, cmp, x) {
		return false
	}
	pass.Report(analysis.Diagnostic{
		Pos:            nilCheck.Pos(),
		End:            nilCheck.End(),
		Message:        "redundant nil check; " + msg,
		SuggestedFixes: fix(file, nilCheck, other),
	})
	return true
}

// nilOperand returns the value e compares with nil, when e is the nil check
// that op can drop: != nil under &&, == nil under ||.
func nilOperand(pass *analysis.Pass, op token.Token, e ast.Expr) (ast.Expr, bool) {
	want := token.NEQ
	if op == token.LOR {
		want = token.EQL
	}
	cmp, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || cmp.Op != want {
		return nil, false
	}
	if isNil(pass, cmp.Y) {
		return ast.Unparen(cmp.X), true
	}
	if isNil(pass, cmp.X) {
		return ast.Unparen(cmp.Y), true
	}
	return nil, false
}

// isNil reports whether e is the predeclared nil.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("nil")
}

// effect reports whether e may produce a different value the second time. A
// call and a channel receive both can.
func effect(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			// A conversion is not a call.
			if !pass.TypesInfo.Types[n.Fun].IsType() {
				found = true
			}
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				found = true
			}
		}
		return !found
	})
	return found
}

// message returns the tail of the diagnostic for x, and "" when len of a nil
// x is not zero. len of a nil pointer to an array is the array length.
func message(pass *analysis.Pass, x ast.Expr) string {
	t := pass.TypesInfo.TypeOf(x)
	if t == nil {
		return ""
	}
	if tp, ok := t.(*types.TypeParam); ok {
		if !setOnly(tp.Constraint()) {
			return ""
		}
		return "len is zero for a nil value"
	}
	kind := oneKind(t)
	if kind == "" {
		return ""
	}
	return "len is zero for a nil " + kind
}

// oneKind returns the word for t when t is a slice, a map or a channel, and ""
// for anything else.
func oneKind(t types.Type) string {
	switch t.Underlying().(type) {
	case *types.Slice:
		return "slice"
	case *types.Map:
		return "map"
	case *types.Chan:
		return "channel"
	}
	return ""
}

// setOnly reports whether every term in t's type set is a slice, a map or a
// channel. An interface that embeds no term, such as any, contributes none:
// len and a nil comparison both reject a type parameter left so unrestricted,
// so no such type reaches this code.
func setOnly(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for embedded := range iface.EmbeddedTypes() {
		union, ok := embedded.(*types.Union)
		if !ok {
			if oneKind(embedded) == "" && !setOnly(embedded) {
				return false
			}
			continue
		}
		for term := range union.Terms() {
			if oneKind(term.Type()) == "" && !setOnly(term.Type()) {
				return false
			}
		}
	}
	return true
}

// redundant reports whether the comparison is false for a length of zero under
// &&, or true under ||.
func redundant(pass *analysis.Pass, op token.Token, cmp *ast.BinaryExpr, x ast.Expr) bool {
	cmpOp, k, ok := compare(pass, cmp, x)
	if !ok {
		return false
	}
	zero := constant.MakeInt64(0)
	return constant.Compare(zero, cmpOp, k) == (op == token.LOR)
}

// compare returns the operator of e and the constant compared with len(x), in
// the order len(x) op k, whatever side of e they were written on.
func compare(pass *analysis.Pass, e *ast.BinaryExpr, x ast.Expr) (token.Token, constant.Value, bool) {
	op, left, right := e.Op, e.X, e.Y
	if !isLenOf(pass, left, x) {
		op, left, right = flipped(op), right, left
	}
	if !isLenOf(pass, left, x) {
		return 0, nil, false
	}
	k, ok := constValue(pass, right)
	return op, k, ok
}

// flipped returns op with its operands swapped.
func flipped(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.GTR:
		return token.LSS
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	}
	return op
}

// isLenOf reports whether e is a call of the builtin len on x.
func isLenOf(pass *analysis.Pass, e, x ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[id] != types.Universe.Lookup("len") {
		return false
	}
	// effect has rejected a call and a receive in the value, so the second
	// read sees the same value.
	return types.ExprString(ast.Unparen(call.Args[0])) == types.ExprString(x)
}

// constValue returns the value of e when e is an integer constant.
func constValue(pass *analysis.Pass, e ast.Expr) (constant.Value, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return nil, false
	}
	return tv.Value, true
}

// fix returns the edits that drop the nil check and the operator joining it to
// the comparison, and no edit when a comment sits in the deleted text, because
// that comment belongs to the author.
func fix(file *ast.File, nilCheck, other ast.Expr) []analysis.SuggestedFix {
	// Each edit runs forward, so a range starts at the earlier end.
	var (
		inner = ast.Unparen(other)
		paren = inner != other
		edits []analysis.TextEdit
	)
	if other.Pos() < nilCheck.Pos() {
		edits = append(edits, analysis.TextEdit{
			Pos: other.End(),
			End: nilCheck.End(),
		})
		if paren {
			edits = append(edits,
				analysis.TextEdit{Pos: other.Pos(), End: inner.Pos()},
				analysis.TextEdit{Pos: inner.End(), End: other.End()},
			)
		}
	} else {
		edits = append(edits, analysis.TextEdit{
			Pos: nilCheck.Pos(),
			End: inner.Pos(),
		})
		if paren {
			edits = append(edits, analysis.TextEdit{
				Pos: inner.End(),
				End: other.End(),
			})
		}
	}
	for _, edit := range edits {
		for _, group := range file.Comments {
			if group.Pos() < edit.End && edit.Pos < group.End() {
				return nil
			}
		}
	}
	return []analysis.SuggestedFix{{
		Message:   "Drop the nil check",
		TextEdits: edits,
	}}
}
