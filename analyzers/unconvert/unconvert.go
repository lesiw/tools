// Package unconvert provides an analysis.Analyzer that reports a conversion
// to a type its operand already has.
//
// The specification's "Conversions" section says an explicit conversion is an
// expression of the form T(x), where T is a type and x is an expression that
// can be converted to T (https://go.dev/ref/spec#Conversions). "Assignability"
// makes a value assignable to a variable of its type
// (https://go.dev/ref/spec#Assignability), and converting a value to the type
// it already has changes nothing. int(n) for an int n and []byte(b) for a
// []byte b are such conversions.
//
// A conversion to a floating-point or complex type is left alone: the spec's
// "Floating-point operators" section says an explicit floating-point
// conversion rounds to the target precision and prevents fused operations
// (https://go.dev/ref/spec#Floating_point_operators), so dropping one can
// change a result even when the operand has the type already. A type
// parameter follows the same rule by a conservative estimate, which the README
// describes. An untyped expression is left alone, since the conversion gives
// it a type. A generated file, including cgo output, is left alone, since an
// edit would change generated text instead of its source.
//
// The fix drops the conversion, and it adds parentheses where regrouping, a
// control clause, or an adjacent operator would change the value or the parse.
// A comment in the dropped text withholds the fix. The README catalogs every
// check and exemption.
//
// The rule is adapted from mdempsky's unconvert.
package unconvert

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unconvert",
	Doc:  "reports a conversion to a type its operand already has",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if generated(pass, file) {
			continue
		}
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			call, ok := n.(*ast.CallExpr)
			if ok {
				check(pass, file, stack, call)
			}
			return true
		})
	}
	return nil, nil
}

// generated reports whether file is generated or cgo output. An edit to it
// would change generated text instead of its source.
func generated(pass *analysis.Pass, file *ast.File) bool {
	if ast.IsGenerated(file) {
		return true
	}
	base := filepath.Base(pass.Fset.File(file.Pos()).Name())
	return base == "_cgo_gotypes.go" || base == "_cgo_import.go" ||
		strings.HasSuffix(base, ".cgo1.go") ||
		strings.HasSuffix(base, "-d")
}

// check reports call when it converts a value to the type that value already
// has.
func check(pass *analysis.Pass, file *ast.File, stack []ast.Node, call *ast.CallExpr) {
	if !conversion(pass, call) {
		return
	}
	fixes := suggested(pass, file, stack, call)
	// Fixing an inner conversion and its outer conversion in one round would
	// drop the context that decides the inner conversion's parentheses, so
	// the inner conversion gets no fix while the outer one has one.
	if nestedFix(pass, file, stack) {
		fixes = nil
	}
	pass.Report(analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        "unnecessary conversion",
		SuggestedFixes: fixes,
	})
}

// nestedFix reports whether call is enclosed by a conversion that has a
// suggested fix.
func nestedFix(pass *analysis.Pass, file *ast.File, stack []ast.Node) bool {
	for i := len(stack) - 2; i >= 1; i-- {
		outer, ok := stack[i].(*ast.CallExpr)
		if ok && suggested(pass, file, stack[:i+1], outer) != nil {
			return true
		}
	}
	return false
}

// parent returns the node above the last node in stack.
func parent(stack []ast.Node) ast.Node {
	if len(stack) < 2 {
		return nil
	}
	return stack[len(stack)-2]
}

// suggested returns the fix that drops call, and nil when call needs none.
func suggested(pass *analysis.Pass, file *ast.File, stack []ast.Node, call *ast.CallExpr) []analysis.SuggestedFix {
	if !conversion(pass, call) {
		return nil
	}
	var (
		arg  = call.Args[0]
		wrap = parens(stack, call, arg, header(stack))
	)
	return fix(file, call, arg, wrap)
}

// conversion reports whether call is a conversion of a value that already has
// the conversion's type.
func conversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 || call.Ellipsis != token.NoPos {
		return false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return false
	}
	arg := call.Args[0]
	if floating(tv.Type) || untyped(pass, arg) {
		return false
	}
	typ := pass.TypesInfo.TypeOf(arg)
	return typ != nil && types.Identical(tv.Type, typ)
}

// untyped reports whether e is written as an untyped value: a literal, an
// untyped constant, nil, a comparison, real, imag, complex, min, max over
// untyped arguments, or an operator expression over untyped values. A shift
// follows the untypedness of its left operand. A conversion gives such a value
// a type, so it is not redundant.
func untyped(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		if pass.TypesInfo.Uses[e] == types.Universe.Lookup("nil") {
			return true
		}
		return untypedConst(pass.TypesInfo.ObjectOf(e))
	case *ast.SelectorExpr:
		return untypedConst(pass.TypesInfo.Uses[e.Sel])
	case *ast.UnaryExpr:
		switch e.Op {
		case token.ADD, token.SUB, token.NOT, token.XOR:
			return untyped(pass, e.X)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.SHL, token.SHR:
			return untyped(pass, e.X)
		case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ,
			token.GEQ:
			// A comparison yields an untyped boolean whatever its operands.
			return true
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM,
			token.AND, token.OR, token.XOR, token.AND_NOT,
			token.LAND, token.LOR:
			return untyped(pass, e.X) && untyped(pass, e.Y)
		}
	case *ast.CallExpr:
		return untypedCall(pass, e)
	}
	return false
}

// untypedConst reports whether obj is a constant of an untyped type.
func untypedConst(obj types.Object) bool {
	c, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	b, ok := c.Type().(*types.Basic)
	return ok && b.Info()&types.IsUntyped != 0
}

// untypedCall reports whether call is a builtin whose result is untyped when
// its arguments are.
func untypedCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	switch pass.TypesInfo.Uses[id] {
	case types.Universe.Lookup("real"), types.Universe.Lookup("imag"):
		return len(call.Args) == 1 && untyped(pass, call.Args[0])
	case types.Universe.Lookup("complex"):
		return len(call.Args) == 2 && untyped(pass, call.Args[0]) &&
			untyped(pass, call.Args[1])
	case types.Universe.Lookup("min"), types.Universe.Lookup("max"):
		for _, arg := range call.Args {
			if !untyped(pass, arg) {
				return false
			}
		}
		return len(call.Args) != 0
	}
	return false
}

// floating reports whether t is a floating-point or complex type, or a type
// parameter whose type set may hold one.
func floating(t types.Type) bool {
	t = types.Unalias(t)
	if param, ok := t.(*types.TypeParam); ok {
		return floatingConstraint(param.Constraint())
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsFloat|types.IsComplex) != 0
}

// floatingConstraint reports whether the type set of c may hold a
// floating-point or complex type.
func floatingConstraint(c types.Type) bool {
	if union, ok := c.(*types.Union); ok {
		for term := range union.Terms() {
			if floating(term.Type()) {
				return true
			}
		}
		return false
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return floating(c)
	}
	// An interface with no type term admits any type that has its methods,
	// which includes a floating-point or complex type.
	if iface.NumEmbeddeds() == 0 {
		return true
	}
	// The type set of an interface is the intersection of its embedded
	// elements, so one element that admits no such type settles it.
	for embedded := range iface.EmbeddedTypes() {
		if !floatingConstraint(embedded) {
			return false
		}
	}
	return true
}

// header reports whether stack holds call in the header of an if, for, range,
// or switch statement, outside any call argument, parenthesized expression, or
// other delimited subexpression.
func header(stack []ast.Node) bool {
	for i := len(stack) - 1; i > 0; i-- {
		parent, child := stack[i-1], stack[i]
		switch n := parent.(type) {
		case *ast.IfStmt:
			return before(child, n.Body.Lbrace)
		case *ast.ForStmt:
			return before(child, n.Body.Lbrace)
		case *ast.RangeStmt:
			return before(child, n.Body.Lbrace)
		case *ast.SwitchStmt:
			return before(child, n.Body.Lbrace)
		case *ast.TypeSwitchStmt:
			return before(child, n.Body.Lbrace)
		case *ast.BlockStmt, *ast.ParenExpr, *ast.CompositeLit:
			return false
		case *ast.CallExpr:
			if child != n.Fun {
				return false
			}
		case *ast.IndexExpr:
			if child != n.X {
				return false
			}
		case *ast.SliceExpr:
			if child != n.X {
				return false
			}
		case *ast.TypeAssertExpr:
			if child != n.X {
				return false
			}
		case *ast.FuncLit:
			if child == n.Body {
				return false
			}
		}
	}
	return false
}

// before reports whether child ends before brace.
func before(child ast.Node, brace token.Pos) bool {
	return child.End() <= brace
}

// parens reports whether the fix has to put parentheses around arg when it
// takes the place of call.
func parens(stack []ast.Node, call *ast.CallExpr, arg ast.Expr, header bool) bool {
	// A composite literal written as a type name would leave its brace to be
	// read as the block of a control clause.
	if header && exposedComposite(arg) {
		return true
	}
	p, ok := parent(stack).(ast.Expr)
	if !ok {
		return false
	}
	// A literal, an identifier, a selector, a call, an index, a slice, a type
	// assertion, and a parenthesized expression all bind tighter than the
	// operators below, and none needs the parentheses.
	prec := precedence(arg)
	if prec < 0 {
		return false
	}
	if postfix(p, call) || fuses(stack, call, arg) {
		return true
	}
	if precedence(p) > prec {
		return true
	}
	return joins(p, call, arg, prec)
}

// postfix reports whether call is the operand of a selector, an index, a
// slice, a type assertion, or a call.
func postfix(parent ast.Expr, call *ast.CallExpr) bool {
	switch p := parent.(type) {
	case *ast.SelectorExpr:
		return p.X == call
	case *ast.IndexExpr:
		return p.X == call
	case *ast.IndexListExpr:
		return p.X == call
	case *ast.SliceExpr:
		return p.X == call
	case *ast.TypeAssertExpr:
		return p.X == call
	case *ast.CallExpr:
		return p.Fun == call
	}
	return false
}

// fuses reports whether arg would join the operator before call into another
// token, as --n does in c-int(-n).
func fuses(stack []ast.Node, call *ast.CallExpr, arg ast.Expr) bool {
	next, ok := firstToken(arg)
	if !ok {
		return false
	}
	for i := len(stack) - 1; i > 0; i-- {
		parent, child := stack[i-1], stack[i]
		switch p := parent.(type) {
		case *ast.BinaryExpr:
			if p.Y == child {
				return adjacent(p.Op, p.OpPos, call) && merges(p.Op, next)
			}
		case *ast.UnaryExpr:
			if p.X == child {
				return adjacent(p.Op, p.OpPos, call) && merges(p.Op, next)
			}
		case *ast.StarExpr:
			if p.X == child {
				return adjacent(token.MUL, p.Star, call) &&
					merges(token.MUL, next)
			}
		case *ast.BlockStmt, *ast.ParenExpr, *ast.CompositeLit,
			*ast.KeyValueExpr:
			return false
		case *ast.CallExpr:
			if p.Fun != child {
				return false
			}
		case *ast.IndexExpr:
			if p.X != child {
				return false
			}
		case *ast.IndexListExpr:
			if p.X != child {
				return false
			}
		case *ast.SliceExpr:
			if p.X != child {
				return false
			}
		case *ast.TypeAssertExpr:
			if p.X != child {
				return false
			}
		}
	}
	return false
}

// adjacent reports whether op, written at pos, ends where call begins.
func adjacent(op token.Token, pos token.Pos, call *ast.CallExpr) bool {
	return pos+token.Pos(len(op.String())) == call.Pos()
}

// merges reports whether op followed by next forms another token.
func merges(op token.Token, next string) bool {
	switch {
	case op == token.ADD && next == "+":
		return true
	case op == token.SUB && next == "-":
		return true
	case op == token.QUO && (next == "*" || next == "/"):
		return true
	case op == token.LSS && (next == "-" || next == "<-"):
		return true
	case op == token.AND && (next == "&" || next == "^"):
		return true
	}
	return false
}

// firstToken returns the first operator token of e when e starts with one.
func firstToken(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.UnaryExpr:
		return e.Op.String(), true
	case *ast.StarExpr:
		return token.MUL.String(), true
	case *ast.BinaryExpr:
		return firstToken(e.X)
	}
	return "", false
}

// joins reports whether arg needs parentheses as the right operand of a binary
// operator at the same precedence as arg.
func joins(parent ast.Expr, call *ast.CallExpr, arg ast.Expr, prec int) bool {
	bin, ok := parent.(*ast.BinaryExpr)
	if !ok || bin.Y != call || precedence(bin) != prec {
		return false
	}
	// One operator repeated on both sides groups the expression the same way
	// when the operator is associative.
	argBin, same := arg.(*ast.BinaryExpr)
	return !same || argBin.Op != bin.Op || !associative(bin.Op)
}

// exposedComposite reports whether e holds a composite literal written as a
// type name where a control clause could read its brace as the block. A call
// argument and a parenthesized expression protect the literals inside them.
func exposedComposite(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.CompositeLit:
		return typeName(e.Type)
	case *ast.ParenExpr:
		return false
	case *ast.CallExpr:
		return exposedComposite(e.Fun)
	case *ast.UnaryExpr:
		return exposedComposite(e.X)
	case *ast.StarExpr:
		return exposedComposite(e.X)
	case *ast.BinaryExpr:
		return exposedComposite(e.X) || exposedComposite(e.Y)
	case *ast.SelectorExpr:
		return exposedComposite(e.X)
	case *ast.IndexExpr:
		return exposedComposite(e.X)
	case *ast.IndexListExpr:
		return exposedComposite(e.X)
	case *ast.SliceExpr:
		return exposedComposite(e.X)
	case *ast.TypeAssertExpr:
		return exposedComposite(e.X)
	}
	return false
}

// typeName reports whether t is a type name, with type arguments if any.
func typeName(t ast.Expr) bool {
	switch ast.Unparen(t).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return true
	}
	return false
}

// precedence returns the binding strength of e, and -1 for an expression that
// binds tighter than any unary or binary operator.
func precedence(e ast.Expr) int {
	switch e := e.(type) {
	case *ast.UnaryExpr:
		return token.UnaryPrec
	case *ast.StarExpr:
		return token.UnaryPrec
	case *ast.BinaryExpr:
		return e.Op.Precedence()
	}
	return -1
}

// associative reports whether op groups its operands the same way on either
// side. The conversions this rule reports never have a floating-point or
// complex type, where addition and multiplication would not qualify.
func associative(op token.Token) bool {
	switch op {
	case token.ADD, token.MUL, token.AND, token.OR, token.XOR,
		token.LAND, token.LOR:
		return true
	}
	return false
}

// fix returns the edit that drops the conversion and any parentheses the
// operand needs, and no fix when a comment sits in the text the edit removes.
func fix(file *ast.File, call *ast.CallExpr, arg ast.Expr, parens bool) []analysis.SuggestedFix {
	var (
		head, tail = "", ""
		edits      []analysis.TextEdit
	)
	if parens {
		head, tail = "(", ")"
	}
	edits = []analysis.TextEdit{{
		Pos:     call.Pos(),
		End:     arg.Pos(),
		NewText: []byte(head),
	}, {
		Pos:     arg.End(),
		End:     call.End(),
		NewText: []byte(tail),
	}}
	for _, edit := range edits {
		for _, group := range file.Comments {
			if group.Pos() < edit.End && edit.Pos < group.End() {
				return nil
			}
		}
	}
	return []analysis.SuggestedFix{{
		Message:   "Drop the conversion",
		TextEdits: edits,
	}}
}
