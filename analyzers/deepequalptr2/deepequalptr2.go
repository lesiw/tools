// Package deepequalptr2 provides an analysis.Analyzer that reports a
// reflect.DeepEqual call on two pointers, including unsafe.Pointer values.
//
// The reflect package documentation for DeepEqual says that pointer values are
// deeply equal "if they are equal using Go's == operator or if they point to
// deeply equal values" (https://pkg.go.dev/reflect#DeepEqual). Under the
// second clause, DeepEqual returns true for two distinct pointers of the same
// type that point to deeply equal values. To test pointer identity, use ==.
//
//	func same(left, right *int) bool {
//		return reflect.DeepEqual(left, right)
//	}
//
// The rule reports a direct call to reflect.DeepEqual in three cases: both
// arguments have the same pointer type and a plain pointee, both arguments
// have different pointer types that are assignable, or both arguments have the
// same unsafe.Pointer type.
//
// A pointee is plain when its underlying type is a basic type (including
// unsafe.Pointer), a channel, a zero-length array, or an array or struct of
// plain types. In the same-type case, the rule leaves the call alone when the
// pointee is uncomparable, except when every uncomparable part is inside a
// zero-length array; when the pointee's underlying type is a pointer (other
// than unsafe.Pointer) or an interface, or a struct or array whose compared
// elements contain either,
// unless the pointer or interface is inside a zero-length array; when the
// pointee is a type parameter, or a struct or array whose compared elements
// contain one, unless the type parameter is inside a zero-length array; or
// when the pointee has an Equal method whose only parameter is the pointee
// type or a pointer to it and whose only result is a boolean. The method is
// the author's equality test, and the pointer comparison would not call it. A
// slice, map, or function has no == comparison, so DeepEqual compares the
// pointees with its own rules.
//
// An argument of type parameter or interface type is not a pointer, and
// neither is an untyped nil, so a call with any of them is left alone. When
// the pointer types differ and one is assignable to the other,
// DeepEqual is always false, so the rule reports the call whatever the
// pointee. When both arguments have the same unsafe.Pointer type, the call is
// reported; DeepEqual compares the values with ==, so the fix changes nothing.
// A call through a function value is left alone. A test file is skipped.
//
// The fix replaces the call with the pointer comparison. For two pointers of
// the same type, it changes the answer for distinct pointers whose pointees
// are deeply equal. For two assignable pointer types that are not identical,
// it changes the answer for the same pointer as well. For the same
// unsafe.Pointer type, it changes nothing. To keep the comparison of the
// pointees, write *left == *right where neither pointer can be nil and the
// pointee type is comparable; otherwise bind reflect.DeepEqual to a variable.
// The fix is withheld where a comment
// lies in the text it would replace, where the call or the printed form of an
// argument spans lines, and where the call is the expression of an expression
// statement or the call of a go or defer statement. The fix parenthesizes the
// comparison under a unary operator, as an operand of == or !=, and in an if,
// for, or switch header when an argument contains a composite literal.
//
// The README lists every check and exemption.
//
// The rule is adapted from staticcheck's SA1021, which reports a net.IP
// comparison by its bytes.
package deepequalptr2

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "deepequalptr2",
	Doc:  "reports reflect.DeepEqual on two pointers, including unsafe.Pointer",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(name, "_test.go") {
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
			if ok && isDeepEqual(pass, call) {
				check(pass, file, call, stack)
			}
			return true
		})
	}
	return nil, nil
}

// isDeepEqual reports whether call is a call to reflect.DeepEqual.
func isDeepEqual(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "reflect" &&
		fn.Name() == "DeepEqual"
}

// check reports call for a same-type pointer pair with a plain pointee, an
// assignable pair of different pointer types, or a pair of unsafe.Pointer
// values.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stack []ast.Node) {
	if len(call.Args) != 2 {
		return
	}
	var (
		left  = pass.TypesInfo.TypeOf(call.Args[0])
		right = pass.TypesInfo.TypeOf(call.Args[1])
	)
	if left == nil || right == nil {
		return
	}
	leftPointer, leftOK := types.Unalias(left).Underlying().(*types.Pointer)
	rightPointer, rightOK := types.Unalias(right).Underlying().(*types.Pointer)
	var message string
	changes := true
	switch {
	case leftOK && rightOK:
		if !assignable(left, right) {
			return
		}
		if types.Identical(left, right) {
			if !pointerReportable(pass, leftPointer, rightPointer) {
				return
			}
			message = "reflect.DeepEqual on two pointers compares what they point to; compare the pointers"
		}
	case unsafePointers(left, right):
		message = "reflect.DeepEqual on two pointers; compare the pointers"
		changes = false
	default:
		return
	}
	if !types.Identical(left, right) {
		message = "reflect.DeepEqual on values of different types is always false; compare the pointers"
		changes = true
	}
	diag := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: message,
	}
	if oneLine(pass, call) {
		if fix, ok := replacement(file, call, stack, changes); ok {
			diag.SuggestedFixes = []analysis.SuggestedFix{fix}
		}
	}
	pass.Report(diag)
}

// oneLine reports whether call starts and ends on one line.
func oneLine(pass *analysis.Pass, call *ast.CallExpr) bool {
	var (
		start = pass.Fset.Position(call.Pos())
		end   = pass.Fset.Position(call.End())
	)
	return start.Line == end.Line
}

// assignable reports whether either type is assignable to the other.
func assignable(left, right types.Type) bool {
	return types.AssignableTo(left, right) || types.AssignableTo(right, left)
}

// pointerReportable reports whether both pointees are plain and neither has
// an Equal method.
func pointerReportable(pass *analysis.Pass, left, right *types.Pointer) bool {
	return plain(left.Elem()) && plain(right.Elem()) &&
		!equalPointees(pass, left, right)
}

// equalPointees reports whether either pointee has an Equal method.
func equalPointees(pass *analysis.Pass, left, right *types.Pointer) bool {
	return hasEqual(pass, left.Elem()) || hasEqual(pass, right.Elem())
}

// hasEqual reports whether t has an Equal method whose only parameter is t or
// a pointer to t and whose only result is a boolean.
func hasEqual(pass *analysis.Pass, t types.Type) bool {
	var (
		obj, _, _ = types.LookupFieldOrMethod(t, true, pass.Pkg, "Equal")
		fn, ok    = obj.(*types.Func)
	)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	if sig.Params().Len() != 1 || sig.Results().Len() != 1 {
		return false
	}
	if !sameOrPointer(sig.Params().At(0).Type(), t) {
		return false
	}
	basic, ok := sig.Results().At(0).Type().Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsBoolean != 0
}

// sameOrPointer reports whether candidate is t or a pointer to t.
func sameOrPointer(candidate, t types.Type) bool {
	return types.Identical(candidate, t) ||
		types.Identical(candidate, types.NewPointer(t))
}

// unsafePointers reports whether both arguments have a type whose underlying
// type is unsafe.Pointer and one type is assignable to the other.
func unsafePointers(left, right types.Type) bool {
	return unsafePointer(left) && unsafePointer(right) &&
		assignable(left, right)
}

// unsafePointer reports whether t has an underlying type that is
// unsafe.Pointer.
func unsafePointer(t types.Type) bool {
	basic, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && basic.Kind() == types.UnsafePointer
}

// plain reports whether t has an underlying type that is basic, a channel, a
// zero-length array, or an array or struct of plain types.
func plain(t types.Type) bool {
	switch t := types.Unalias(t).Underlying().(type) {
	case *types.Basic, *types.Chan:
		return true
	case *types.Array:
		return t.Len() == 0 || plain(t.Elem())
	case *types.Struct:
		for field := range t.Fields() {
			if !plain(field.Type()) {
				return false
			}
		}
		return true
	}
	return false
}

// replacement returns the suggested fix for the pointer comparison.
func replacement(file *ast.File, call *ast.CallExpr, stack []ast.Node, changes bool) (analysis.SuggestedFix, bool) {
	if statementCall(stack) {
		return analysis.SuggestedFix{}, false
	}
	if commented(file, call.Pos(), call.End()) {
		return analysis.SuggestedFix{}, false
	}
	left, ok := astexpr.OneLine(ast.Unparen(call.Args[0]))
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	right, ok := astexpr.OneLine(ast.Unparen(call.Args[1]))
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	text := left + " == " + right
	if needsParens(stack) || needsHeaderParens(stack, call.Args) {
		text = "(" + text + ")"
	}
	message := "Compare the pointers"
	if changes {
		message += " (changes behavior)"
	}
	return analysis.SuggestedFix{
		Message: message,
		TextEdits: []analysis.TextEdit{{
			Pos:     call.Pos(),
			End:     call.End(),
			NewText: []byte(text),
		}},
	}, true
}

// statementCall reports whether call is the expression of an expression
// statement or the call of a go or defer statement.
func statementCall(stack []ast.Node) bool {
	for i := len(stack) - 2; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ParenExpr:
			continue
		case *ast.ExprStmt, *ast.GoStmt, *ast.DeferStmt:
			return true
		default:
			return false
		}
	}
	return false
}

// needsHeaderParens reports whether the replacement holds a composite literal
// in an if, for, or switch header outside any brackets.
func needsHeaderParens(stack []ast.Node, args []ast.Expr) bool {
	if !hasComposite(args[0]) && !hasComposite(args[1]) {
		return false
	}
	for i := len(stack) - 2; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.ParenExpr, *ast.CallExpr, *ast.IndexExpr,
			*ast.IndexListExpr, *ast.SliceExpr, *ast.CompositeLit,
			*ast.KeyValueExpr:
			return false
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt:
			return true
		case *ast.BlockStmt:
			return false
		}
	}
	return false
}

// hasComposite reports whether e contains a composite literal.
func hasComposite(e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.CompositeLit); ok {
			found = true
		}
		return !found
	})
	return found
}

// needsParens reports whether the replacement needs parentheses around it.
func needsParens(stack []ast.Node) bool {
	if len(stack) < 2 {
		return false
	}
	switch parent := stack[len(stack)-2].(type) {
	case *ast.UnaryExpr:
		return true
	case *ast.BinaryExpr:
		return parent.Op == token.EQL || parent.Op == token.NEQ
	}
	return false
}

// commented reports whether a comment lies in the text a fix would replace.
func commented(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() >= from && comment.Pos() < to {
				return true
			}
		}
	}
	return false
}
