package evalorder

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type span struct {
	pos, end token.Pos
}

// holds reports whether pos is inside s.
func (s span) holds(pos token.Pos) bool { return s.pos <= pos && pos < s.end }

// chain is a chain of && or ||, and the position of its operator.
type chain struct {
	span
	op token.Pos
}

// order collects the source ranges whose evaluation the spec fixes.
type order struct {
	calls  []span
	chains []chain
}

// spans records the calls and the chains of && and || in n.
func spans(pass *analysis.Pass, n ast.Node) (o order) {
	eval(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if _, ok := signature(pass, n); !ok {
				break // a conversion and a builtin order nothing
			}
			o.calls = append(o.calls, span{n.Pos(), n.End()})
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				o.chains = append(o.chains, chain{
					span: span{n.Pos(), n.End()},
					op:   n.OpPos,
				})
			}
		}
		return true
	})
	return o
}

// ordered reports whether the spec fixes the order of the address taken at
// addr and the read at read. Calls, including the innermost call that holds
// the address, run left to right after their arguments, so a read inside a
// call that ends no later than that one comes first. A chain of && or || runs
// its operands left to right, which settles the order of a read on one side
// and an address on the other whichever side each is on.
func (o order) ordered(addr, read token.Pos) bool {
	for _, c := range o.chains {
		if !c.holds(addr) || !c.holds(read) {
			continue
		}
		if (read < c.op) != (addr < c.op) {
			return true
		}
	}
	end := o.callEnd(addr)
	for _, c := range o.calls {
		if c.holds(read) && c.end <= end {
			return true
		}
	}
	return false
}

// callEnd returns the end of the innermost call that contains pos.
func (o order) callEnd(pos token.Pos) token.Pos {
	end := token.NoPos
	for _, c := range o.calls {
		if c.holds(pos) && (end == token.NoPos || c.end < end) {
			end = c.end
		}
	}
	return end
}

// eval calls f for the nodes of n that evaluating it reaches. It enters the
// body of a function literal that the expression calls, and not the body of
// one that it does not.
func eval(n ast.Node, f func(ast.Node) bool) {
	invoked := map[*ast.FuncLit]struct{}{}
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			if _, ok := invoked[n]; !ok {
				return false
			}
		case *ast.CallExpr:
			if lit, ok := ast.Unparen(n.Fun).(*ast.FuncLit); ok {
				invoked[lit] = struct{}{}
			}
		}
		return f(n)
	})
}
