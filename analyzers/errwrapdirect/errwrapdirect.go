// Package errwrapdirect provides an analysis.Analyzer that reports a
// fmt.Errorf call that wraps an error without a message.
//
// The fmt package documentation says that a %w verb with an error operand
// returns an error whose Unwrap method returns the operand
// (https://pkg.go.dev/fmt#Errorf), and the errors package documentation
// describes fmt.Errorf with %w as the way to add context to an error
// (https://pkg.go.dev/errors#pkg-overview). A format of exactly %w adds no
// text, so the message is the operand's message and the result unwraps to the
// operand.
//
// Only a literal format string is examined, and its value has to be exactly
// %w. A format with any other value is left alone, and so is a format held in
// a constant or a variable. A call with a missing, extra, or spread argument
// is left alone. The argument has to be assignable to error.
//
// A %w verb formats its operand like %v, so an operand that implements
// fmt.Formatter is formatted by the Format method. An argument whose static
// type implements fmt.Formatter is left alone; a Formatter behind the error
// interface is not known.
//
// The fix replaces the call with the argument. When the argument's static type
// is exactly error, the argument can stand wherever the call stood. An
// argument of any other type is converted to error, so the replacement has
// the call's static type; the fix is withheld where error is shadowed. A unary
// argument that would bind to a selector or a type assertion is parenthesized.
// The fix is also withheld in an expression statement, a go statement, and a
// defer statement, where an error value is not a statement. It is withheld
// where the call is directly compared with == or !=, directly used as a switch
// tag or case, directly used as a map key, or in the init statement of an if,
// switch (including a type switch), or for. A conversion, a type assertion,
// and a composite literal are followed. A comparison can panic when both
// operands hold the same non-comparable dynamic type, which the replacement
// can create.
//
// The replacement is not the same value. A comparison with == that was false
// for the wrapper can be true for the operand, and the result has one fewer
// Unwrap layer. A type assertion, %T, or reflect.TypeOf reports the operand's
// type where it reported the wrapper's. A wrapper around a sentinel may be
// deliberate for exactly that reason. The rule reports the call anyway.
//
// A nil literal and a conversion of nil are left alone. Wrapping nil gives a
// non-nil error, and a typed nil can make the operand's Error method panic.
// Any other expression is not inspected for nilness, so an error variable
// that is nil at run time reports and the fix returns nil. A comment in the
// text the fix removes withholds the fix. A call in a test file is checked
// like any other call; //ignore on the call keeps one a test needs.
//
// The rule is adapted from staticcheck's S1028.
package errwrapdirect

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errwrapdirect",
	Doc:  "reports a fmt.Errorf call that wraps an error without a message",
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
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call, stack)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a fmt.Errorf call whose only format verb is %w and whose
// argument is an error that can stand in its place.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stack []ast.Node) {
	if call.Ellipsis.IsValid() || len(call.Args) != 2 {
		return
	}
	if !errorf(pass, call.Fun) {
		return
	}
	format, ok := formatValue(call.Args[0])
	if !ok || format != "%w" {
		return
	}
	arg := call.Args[1]
	t := pass.TypesInfo.TypeOf(arg)
	if !assignableToError(t) || isNil(pass, arg) || formatter(pass, t) {
		return
	}
	convert := !types.Identical(t, errorType)
	var fixes []analysis.SuggestedFix
	if !statementNeedsCall(stack, call) {
		if !comparisonContext(pass, stack, call) {
			if !convert || errorBuiltin(pass, file, call.Pos()) {
				fixes = fix(file, call, arg, parent(stack), convert)
			}
		}
	}
	pass.Report(analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        "error is wrapped without a message",
		SuggestedFixes: fixes,
	})
}

// errorf reports whether fun is fmt.Errorf.
func errorf(pass *analysis.Pass, fun ast.Expr) bool {
	var obj types.Object
	switch fun := ast.Unparen(fun).(type) {
	case *ast.SelectorExpr:
		obj = pass.TypesInfo.Uses[fun.Sel]
	case *ast.Ident:
		obj = pass.TypesInfo.Uses[fun]
	}
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "fmt" {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil && fn.Name() == "Errorf"
}

// formatValue returns the value of a literal string argument.
func formatValue(arg ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(arg).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

var errorType = types.Universe.Lookup("error").Type()

// assignableToError reports whether a value of type t can stand where the
// call's error result stands.
func assignableToError(t types.Type) bool {
	return t != nil && types.AssignableTo(t, errorType)
}

// formatter reports whether t implements fmt.Formatter.
func formatter(pass *analysis.Pass, t types.Type) bool {
	iface := formatterInterface(pass)
	return t != nil && iface != nil && types.Implements(t, iface)
}

// formatterInterface returns fmt.Formatter from the package or an import.
func formatterInterface(pass *analysis.Pass) *types.Interface {
	if pass.Pkg.Path() == "fmt" {
		if obj := pass.Pkg.Scope().Lookup("Formatter"); obj != nil {
			iface, _ := obj.Type().Underlying().(*types.Interface)
			return iface
		}
	}
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() != "fmt" {
			continue
		}
		obj := pkg.Scope().Lookup("Formatter")
		if obj == nil {
			return nil
		}
		iface, _ := obj.Type().Underlying().(*types.Interface)
		return iface
	}
	return nil
}

// errorBuiltin reports whether error names the predeclared type at pos.
func errorBuiltin(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	root := pass.TypesInfo.Scopes[file]
	if root == nil {
		return false
	}
	_, obj := root.Innermost(pos).LookupParent("error", pos)
	return obj == types.Universe.Lookup("error")
}

// isNil reports whether e is nil at compile time. A conversion of nil has a
// type, and wrapping it need not give the operand's message.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[e] == types.Universe.Lookup("nil")
	case *ast.CallExpr:
		if len(e.Args) != 1 || !pass.TypesInfo.Types[e.Fun].IsType() {
			return false
		}
		return isNil(pass, e.Args[0])
	}
	return false
}

// statementNeedsCall reports whether call is the expression of a statement
// that takes a call, where an error value cannot stand.
func statementNeedsCall(stack []ast.Node, call ast.Expr) bool {
	var (
		i     = len(stack) - 1
		outer = call
	)
	for i > 0 {
		paren, ok := stack[i-1].(*ast.ParenExpr)
		if !ok {
			break
		}
		outer, i = paren, i-1
	}
	if i < 2 {
		return false
	}
	switch stmt := stack[i-1].(type) {
	case *ast.ExprStmt:
		return stmt.X == outer
	case *ast.GoStmt:
		return stmt.Call == outer
	case *ast.DeferStmt:
		return stmt.Call == outer
	}
	return false
}

// initContains reports whether call is inside init.
func initContains(init ast.Stmt, call ast.Expr) bool {
	return init != nil && init.Pos() <= call.Pos() && call.End() <= init.End()
}

// parensFree reports whether arg's parentheses can be dropped without losing
// a comment.
func parensFree(file *ast.File, arg ast.Expr) bool {
	inner := ast.Unparen(arg)
	if inner == arg {
		return true
	}
	return !commented(file, arg.Pos(), inner.Pos()) &&
		!commented(file, inner.End(), arg.End())
}

// comparisonContext reports whether the result of call is compared, used as a
// switch tag or case, or used as a map key, where an interface holding a
// non-comparable error can panic.
func comparisonContext(pass *analysis.Pass, stack []ast.Node, call ast.Expr) bool {
	if inStatementInit(stack, call) {
		return true
	}
	var (
		i     = len(stack) - 1
		outer = call
	)
	for i > 0 {
		next, ok := transparent(pass, stack[i-1], outer)
		if !ok {
			break
		}
		outer, i = next, i-1
	}
	if i < 2 {
		return false
	}
	switch parent := stack[i-1].(type) {
	case *ast.BinaryExpr:
		if parent.Op != token.EQL && parent.Op != token.NEQ {
			return false
		}
		return parent.X == outer || parent.Y == outer
	case *ast.SwitchStmt:
		return parent.Tag == outer
	case *ast.CaseClause:
		return slices.Contains(parent.List, outer)
	case *ast.IndexExpr:
		return parent.Index == outer
	case *ast.KeyValueExpr:
		return parent.Key == outer
	case *ast.CallExpr:
		return deleteKey(pass, parent, outer)
	}
	return false
}

// inStatementInit reports whether call is inside a statement init.
func inStatementInit(stack []ast.Node, call ast.Expr) bool {
	for _, n := range stack {
		var init ast.Stmt
		switch stmt := n.(type) {
		case *ast.SwitchStmt:
			init = stmt.Init
		case *ast.IfStmt:
			init = stmt.Init
		case *ast.ForStmt:
			init = stmt.Init
		case *ast.TypeSwitchStmt:
			init = stmt.Init
		}
		if initContains(init, call) {
			return true
		}
	}
	return false
}

// transparent returns the expression parent wraps around outer, and reports
// whether parent is a node that passes outer through.
func transparent(pass *analysis.Pass, parent ast.Node, outer ast.Expr) (ast.Expr, bool) {
	switch parent := parent.(type) {
	case *ast.ParenExpr:
		return parent, true
	case *ast.CallExpr:
		return parent, parent.Fun != outer && conversion(pass, parent, outer)
	case *ast.TypeAssertExpr:
		return parent, parent.X == outer
	case *ast.KeyValueExpr:
		return parent, parent.Value == outer
	case *ast.CompositeLit:
		return parent, slices.Contains(parent.Elts, outer)
	}
	return nil, false
}

// conversion reports whether call converts e to another type.
func conversion(pass *analysis.Pass, call *ast.CallExpr, e ast.Expr) bool {
	return len(call.Args) == 1 && call.Args[0] == e &&
		pass.TypesInfo.Types[call.Fun].IsType()
}

// deleteKey reports whether e is a key argument of the builtin delete.
func deleteKey(pass *analysis.Pass, call *ast.CallExpr, e ast.Expr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok || builtin.Name() != "delete" {
		return false
	}
	return slices.Contains(call.Args[1:], e)
}

// parent returns the node that holds the call, or nil at the top.
func parent(stack []ast.Node) ast.Node {
	if len(stack) < 2 {
		return nil
	}
	return stack[len(stack)-2]
}

// fix returns the edit that drops the call around arg, and nothing when a
// comment sits in the text it removes. An argument of any other type is
// converted to error so the replacement has the call's static type.
func fix(file *ast.File, call *ast.CallExpr, arg ast.Expr, parent ast.Node, convert bool) []analysis.SuggestedFix {
	if commented(file, call.Pos(), arg.Pos()) {
		return nil
	}
	if commented(file, arg.End(), call.End()) {
		return nil
	}
	if parensFree(file, arg) {
		arg = ast.Unparen(arg)
	}
	var (
		left  = analysis.TextEdit{Pos: call.Pos(), End: arg.Pos()}
		right = analysis.TextEdit{Pos: arg.End(), End: call.End()}
	)
	switch {
	case convert:
		left.NewText = []byte("error(")
		right.NewText = []byte(")")
	case needsParens(parent, call, arg):
		left.NewText = []byte("(")
		right.NewText = []byte(")")
	}
	return []analysis.SuggestedFix{{
		Message:   "Drop the wrapper",
		TextEdits: []analysis.TextEdit{left, right},
	}}
}

// needsParens reports whether arg needs parentheses when it takes the place of
// call in parent. A unary expression binds into a selector or a type
// assertion.
func needsParens(parent ast.Node, call, arg ast.Expr) bool {
	if _, ok := arg.(*ast.ParenExpr); ok {
		return false
	}
	switch arg.(type) {
	case *ast.StarExpr, *ast.UnaryExpr:
	default:
		return false
	}
	switch parent := parent.(type) {
	case *ast.SelectorExpr:
		return parent.X == call
	case *ast.TypeAssertExpr:
		return parent.X == call
	}
	return false
}

// commented reports whether a comment overlaps the text between lo and hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
