// Package boolliteral provides an analysis.Analyzer that reports a comparison
// with a boolean literal.
//
// The specification says a comparison yields an untyped boolean value
// (https://go.dev/ref/spec#Comparison_operators), and the logical operators
// yield a result of the same type as their operands
// (https://go.dev/ref/spec#Logical_operators), so the operand can replace the
// comparison.
//
// The comparison is replaced by the operand, with a ! where the comparison
// inverts the operand. A negated binary operand is parenthesized. The README
// catalogs every check and exemption.
//
// The rule is adapted from revive's bool-literal-in-expr.
package boolliteral

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "boolliteral",
	Doc:  "reports a comparison with a boolean literal",
	Run:  run,
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
			if bin, ok := n.(*ast.BinaryExpr); ok {
				check(pass, file, bin, insideConst(stack))
			}
			return true
		})
	}
	return nil, nil
}

// insideConst reports whether the node at the end of stack is in a constant
// declaration.
func insideConst(stack []ast.Node) bool {
	for _, s := range slices.Backward(stack) {
		if decl, ok := s.(*ast.GenDecl); ok && decl.Tok == token.CONST {
			return true
		}
	}
	return false
}

// check reports a comparison with a boolean literal on one side.
func check(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, inConst bool) {
	if bin.Op != token.EQL && bin.Op != token.NEQ {
		return
	}
	literal, operand, ok := sides(pass, bin)
	if !ok || !boolean(pass.TypesInfo.TypeOf(operand)) {
		return
	}
	// == true and != false leave the operand as it is. The other two invert
	// it.
	invert := (bin.Op == token.EQL) != literal
	if !fixable(pass, file, bin, operand, invert, inConst) {
		return
	}
	message := "comparison with true; write the operand"
	if !literal {
		message = "comparison with false; write the operand"
	}
	if invert {
		message += " negated"
	}
	pass.Report(analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: message,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "Write the operand",
			TextEdits: edits(bin, operand, invert),
		}},
	})
}

// sides returns the value of the boolean literal on one side of bin and the
// expression on the other. It reports false when neither side or both sides
// hold a literal.
func sides(pass *analysis.Pass, bin *ast.BinaryExpr) (literal bool, operand ast.Expr, ok bool) {
	if value, is := truth(pass, bin.Y); is {
		if _, both := truth(pass, bin.X); both {
			return false, nil, false
		}
		return value, bin.X, true
	}
	if value, is := truth(pass, bin.X); is {
		return value, bin.Y, true
	}
	return false, nil, false
}

// truth returns the value of the predeclared boolean constant e names, and
// whether e names one. A name that shadows the constant is not one.
func truth(pass *analysis.Pass, e ast.Expr) (value, ok bool) {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false, false
	}
	switch pass.TypesInfo.Uses[id] {
	case types.Universe.Lookup("true"):
		return true, true
	case types.Universe.Lookup("false"):
		return false, true
	}
	return false, false
}

// boolean reports whether t is a boolean type or a type parameter whose type
// set holds only boolean types.
func boolean(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if param, ok := t.(*types.TypeParam); ok {
		return booleanConstraint(param.Constraint())
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

// booleanConstraint reports whether every type in c's type set is boolean.
func booleanConstraint(c types.Type) (found bool) {
	if union, ok := c.(*types.Union); ok {
		if union.Len() == 0 {
			return false
		}
		for term := range union.Terms() {
			if !booleanConstraint(term.Type()) {
				return false
			}
		}
		return true
	}
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return boolean(c)
	}
	for embedded := range iface.EmbeddedTypes() {
		if unrestricted(embedded) {
			continue
		}
		if !booleanConstraint(embedded) {
			return false
		}
		found = true
	}
	return found
}

// unrestricted reports whether an element of a type set names no type of its
// own, as an embedding of comparable or any does. An interface that embeds
// only such elements is unrestricted too.
func unrestricted(t types.Type) bool {
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for embedded := range iface.EmbeddedTypes() {
		if !unrestricted(embedded) {
			return false
		}
	}
	return true
}

// fixable reports whether the operand can stand where the comparison did.
// A negation keeps the operand's type, so either form has that type. An
// untyped comparison result takes any one boolean type in a condition, but a
// logical operator can make two operands share a type, and a constant
// declaration would take the operand's type. The fix is therefore offered for
// an untyped result when the operand is untyped, or when it has the
// predeclared bool type, including an alias, and the comparison is not in a
// constant declaration. A typed result takes the operand when it is assignable
// to that type.
func fixable(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr, operand ast.Expr, invert, inConst bool) bool {
	var (
		result      = pass.TypesInfo.TypeOf(bin)
		operandType = pass.TypesInfo.TypeOf(operand)
	)
	if result == nil || operandType == nil {
		return false
	}
	if isUntypedBool(result) {
		if !isUntypedBool(operandType) {
			if inConst || !plainBool(operandType) {
				return false
			}
		}
	} else if !types.AssignableTo(operandType, result) {
		return false
	}
	pos, end := deleted(bin, operand)
	if commented(file.Comments, pos, end) {
		return false
	}
	return fits(pass, bin, operand, invert)
}

// deleted returns the span the fix removes: the literal, the operator, and
// the space between them.
func deleted(bin *ast.BinaryExpr, operand ast.Expr) (pos, end token.Pos) {
	if operand == bin.X {
		return operand.End(), bin.End()
	}
	return bin.Pos(), operand.Pos()
}

// edits returns the edits that write the operand in place of the comparison.
func edits(bin *ast.BinaryExpr, operand ast.Expr, invert bool) []analysis.TextEdit {
	var (
		pos, end = deleted(bin, operand)
		edit     = analysis.TextEdit{Pos: pos, End: end}
	)
	if !invert {
		return []analysis.TextEdit{edit}
	}
	// A binary operand needs its own parentheses, since ! binds to the first
	// operand of the binary expression rather than to the whole expression.
	openAt, closeAt := "!", ""
	if _, ok := operand.(*ast.BinaryExpr); ok {
		openAt, closeAt = "!(", ")"
	}
	if operand == bin.X {
		edit.NewText = []byte(closeAt)
		return []analysis.TextEdit{
			{Pos: bin.Pos(), End: bin.Pos(), NewText: []byte(openAt)},
			edit,
		}
	}
	edit.NewText = []byte(openAt)
	if closeAt == "" {
		return []analysis.TextEdit{edit}
	}
	return []analysis.TextEdit{
		edit,
		{Pos: operand.End(), End: operand.End(), NewText: []byte(closeAt)},
	}
}

// isUntypedBool reports whether t is the type of an untyped boolean value.
func isUntypedBool(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.UntypedBool
}

// plainBool reports whether t is the predeclared bool type or an alias of it.
func plainBool(t types.Type) bool {
	return types.Identical(types.Unalias(t), types.Typ[types.Bool])
}

// commented reports whether a comment overlaps the span from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}

// fits reports whether the rewritten lines stay within the width limit. A
// rewrite is allowed when a replaced line is already past the limit, even if
// the rewritten line is not shorter.
func fits(pass *analysis.Pass, bin *ast.BinaryExpr, operand ast.Expr, invert bool) (over bool) {
	if linelen.Config.Len <= 0 {
		return true
	}
	file := pass.Fset.File(bin.Pos())
	if file == nil {
		return false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	start := file.Offset(bin.Pos())
	stop := file.Offset(bin.End())
	if start > len(src) || stop > len(src) {
		return false
	}
	start = bytes.LastIndexByte(src[:start], '\n') + 1
	if tail := bytes.IndexByte(src[stop:], '\n'); tail < 0 {
		stop = len(src)
	} else {
		stop += tail
	}
	for line := range bytes.SplitSeq(src[start:stop], []byte("\n")) {
		over = over || !linelen.CheckLine(line)
	}
	out, cursor := make([]byte, 0, stop-start), start
	for _, edit := range edits(bin, operand, invert) {
		lo, hi := file.Offset(edit.Pos), file.Offset(edit.End)
		if lo < cursor || hi < lo || stop < hi {
			return false
		}
		out = append(out, src[cursor:lo]...)
		out = append(out, edit.NewText...)
		cursor = hi
	}
	out = append(out, src[cursor:stop]...)
	for line := range bytes.SplitSeq(out, []byte("\n")) {
		if !linelen.CheckLine(line) {
			return over
		}
	}
	return true
}
