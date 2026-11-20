// Package deepequalptr provides an analysis.Analyzer that reports a
// reflect.DeepEqual call on a type with an Equal method and a pointer field,
// or on two pointers.
//
// The reflect package's DeepEqual documentation says that pointer values are
// deeply equal "if they are equal using Go's == operator or if they point to
// deeply equal values" (https://pkg.go.dev/reflect#DeepEqual). DeepEqual
// compares the value behind a pointer, returns false when the two dynamic
// types differ, and never calls an Equal method. The rule has two arms, one
// for an Equal method and one for == on the pointers themselves.
//
// The Equal arm reports a call whose arguments have the same type. That type
// has an Equal method whose parameter is its own receiver type and whose
// result is a boolean; the receiver may be the type or a pointer to it. A
// pointer field must be reachable from the type.
//
//	type item struct {
//		meta *int
//	}
//
//	func (i item) Equal(other item) bool {
//		return i.meta == other.meta
//	}
//
//	func same(left, right item) bool {
//		return reflect.DeepEqual(left, right)
//	}
//
// DeepEqual compares the values behind meta, and Equal compares the addresses,
// so the two comparisons return different answers on distinct pointers to
// equal values. The message includes the path to the first non-blank pointer
// field in declaration order. The path is read through struct fields, array
// and slice elements, and map values, and adds [] for each container. Pointers
// at the top of the compared type are followed first, so a pointer and a
// pointer to a pointer read the same fields. A zero-length array field is not
// read. A pointer field that is unexported and comes from another package is
// named only as a pointer field.
//
// The arm leaves alone a type with no reachable pointer field, a type that is
// not a struct or a pointer to one, a type whose Equal method takes another
// type or is promoted from an embedded field, and a type whose Equal method
// does not return a boolean. It also leaves alone a call whose same-package
// Equal method dereferences the first direct pointer field, because the two
// comparisons agree there; in another package the arm checks the signature
// only, and it still reports a call whose method reads the same pointee. A
// field whose type is a channel, a function, an interface, or a type parameter
// is not read, even when a type parameter's constraint admits only pointer
// types. The Equal arm leaves alone a call whose arguments have different
// static types, because the dynamic type behind an interface is not known.
//
// The identity arm reports a call whose two arguments are the same pointer
// type with a plain pointee, two assignable pointer types that are not
// identical, or two values of the same unsafe.Pointer type.
//
//	func samePointer(left, right *int) bool {
//		return reflect.DeepEqual(left, right)
//	}
//
// A pointee is plain when its underlying type is a basic type (including
// unsafe.Pointer), a channel, a zero-length array, or an array or struct of
// plain types; a zero-length array is plain whatever it holds, so an
// uncomparable element there is never compared. A call on a pointee that is
// not plain is left alone, as is a call on a pointee with an Equal method
// whose parameter is the pointee type or a pointer to it: the method is the
// author's equality test, and == does not call it. A type with a reachable
// pointer field is not plain, so the identity arm leaves its calls alone; the
// Equal arm reports the call when the Equal method takes its own receiver
// type.
//
// An argument of type parameter or interface type is not a pointer, and
// neither is an untyped nil, so a call with any of them is left alone. A call
// whose arguments are not assignable to each other is left alone as well.
//
// A test file is skipped, and only a direct call to reflect.DeepEqual is
// matched.
//
// The identity arm has a SuggestedFix that replaces the call with the pointer
// comparison. For two pointers of the same type it changes the answer for
// distinct pointers whose pointees are deeply equal. For two assignable
// pointer types that are not identical it also changes the answer when both
// arguments hold the same pointer. For the same unsafe.Pointer type it changes
// nothing. The Equal arm reports only. The repair writes a call to the method,
// whose result the analyzer does not compute, and the receiver's form decides
// whether an argument is a value, an address, or a dereference. Bind
// reflect.DeepEqual to a variable when the structural comparison is meant.
//
// The README catalogs every check and exemption. The Equal arm is inspired by
// staticcheck's SA1021, which reports a net.IP comparison by its bytes instead
// of net.IP.Equal. The identity arm is adapted from the same rule, applied to
// any pointer rather than to net.IP.
package deepequalptr

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/typename"
)

var Analyzer = &analysis.Analyzer{
	Name: "deepequalptr",
	Doc:  "reports DeepEqual on a type with an Equal method and a pointer field, or on two pointers",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go") {
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
			if ok && len(call.Args) == 2 && isDeepEqual(pass, call) {
				check(pass, file, call, stack)
			}
			return true
		})
	}
	return nil, nil
}

// isDeepEqual reports whether call calls reflect.DeepEqual. A call through a
// function value has no static callee and is left alone.
func isDeepEqual(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == "reflect" &&
		fn.Name() == "DeepEqual"
}

// check reports call with whichever arm applies to the two arguments.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stack []ast.Node) {
	var (
		left  = pass.TypesInfo.TypeOf(call.Args[0])
		right = pass.TypesInfo.TypeOf(call.Args[1])
	)
	if left == nil || right == nil {
		return
	}
	diag, ok := equalDiagnostic(pass, call, left, right)
	if !ok {
		diag, ok = identityDiagnostic(pass, file, call, stack, left, right)
	}
	if ok {
		pass.Report(diag)
	}
}

// equalMethod returns the Equal method of t whose only parameter is t or a
// pointer to t and whose only result is a boolean. The method is the author's
// equality test, and reflect.DeepEqual does not call it.
func equalMethod(pass *analysis.Pass, t types.Type) (*types.Func, *types.Signature, bool) {
	var (
		obj, _, _ = types.LookupFieldOrMethod(t, true, pass.Pkg, "Equal")
		fn, ok    = obj.(*types.Func)
	)
	if !ok {
		return nil, nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil, nil, false
	}
	if sig.Params().Len() != 1 || sig.Results().Len() != 1 {
		return nil, nil, false
	}
	if !sameOrPointer(sig.Params().At(0).Type(), t) {
		return nil, nil, false
	}
	basic, ok := sig.Results().At(0).Type().Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsBoolean == 0 {
		return nil, nil, false
	}
	return fn, sig, true
}

func sameOrPointer(candidate, t types.Type) bool {
	return types.Identical(candidate, t) ||
		types.Identical(candidate, types.NewPointer(t))
}

func equalDiagnostic(pass *analysis.Pass, call *ast.CallExpr, left, right types.Type) (analysis.Diagnostic, bool) {
	if !types.Identical(left, right) {
		return analysis.Diagnostic{}, false
	}
	field, fieldVar, ok := pointerField(left)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	recv, fn, ok := equalReceiver(pass, left)
	if !ok || equalDereferences(pass, fn, field) {
		return analysis.Diagnostic{}, false
	}
	msg := "reflect.DeepEqual does not call " + equalName(pass, recv) +
		"; a pointer field is followed to its pointee"
	if fieldVar.Exported() || fieldVar.Pkg() == pass.Pkg {
		msg = "reflect.DeepEqual does not call " + equalName(pass, recv) +
			"; pointer field " + field + " is followed to its pointee"
	}
	return analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: msg,
	}, true
}

// equalReceiver returns the receiver type of the Equal method that applies to
// t, after any pointer is followed. The method takes an argument of its own
// receiver type, so a promoted method whose parameter is the outer type does
// not apply.
func equalReceiver(pass *analysis.Pass, t types.Type) (types.Type, *types.Func, bool) {
	var (
		base        = baseType(t)
		fn, sig, ok = equalMethod(pass, base)
	)
	if !ok || !types.Identical(sig.Recv().Type(), sig.Params().At(0).Type()) {
		return nil, nil, false
	}
	return sig.Recv().Type(), fn, true
}

// equalDereferences reports whether a same-package Equal method dereferences
// the direct pointer field named field.
func equalDereferences(pass *analysis.Pass, fn *types.Func, name string) bool {
	if name == "" || strings.ContainsAny(name, ".[") {
		return false
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			decl, ok := decl.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			if pass.TypesInfo.Defs[decl.Name] != fn {
				continue
			}
			var found bool
			ast.Inspect(decl.Body, func(n ast.Node) bool {
				star, ok := n.(*ast.StarExpr)
				if !ok {
					return true
				}
				sel, ok := ast.Unparen(star.X).(*ast.SelectorExpr)
				if ok && sel.Sel.Name == name {
					found = true
					return false
				}
				return true
			})
			return found
		}
	}
	return false
}

// equalName returns the text that names the Equal method with receiver t.
func equalName(pass *analysis.Pass, t types.Type) string {
	t = types.Unalias(t)
	name := types.TypeString(t, typename.Of(pass))
	if _, ok := t.Underlying().(*types.Pointer); ok {
		return "(" + name + ").Equal"
	}
	return name + ".Equal"
}

// baseType returns the type reached from t by following pointers.
func baseType(t types.Type) types.Type {
	seen := make(map[types.Type]struct{})
	t = types.Unalias(t)
	for {
		ptr, ok := t.Underlying().(*types.Pointer)
		if !ok {
			return t
		}
		if _, ok := seen[t]; ok {
			return t
		}
		seen[t] = struct{}{}
		t = types.Unalias(ptr.Elem())
	}
}

// pointerField returns the path to the first pointer field reachable from t
// through struct fields, array and slice elements, and map values, in
// declaration order. Pointers at the top of t are followed first, since the
// arguments themselves may be pointers.
func pointerField(t types.Type) (string, *types.Var, bool) {
	t = baseType(t)
	if _, ok := t.Underlying().(*types.Struct); !ok {
		return "", nil, false
	}
	return fieldPointer(t, "", nil, make(map[types.Type]struct{}))
}

// fieldPointer returns the path to the first pointer in t and the field that
// holds it. An array, a slice, or a map adds [] to the path. The path is empty
// only at the top, which is a struct.
func fieldPointer(t types.Type, path string, field *types.Var, seen map[types.Type]struct{}) (string, *types.Var, bool) {
	t = types.Unalias(t)
	if t == nil {
		return "", nil, false
	}
	if _, ok := seen[t]; ok {
		return "", nil, false
	}
	seen[t] = struct{}{}
	switch t := t.Underlying().(type) {
	case *types.Pointer:
		if path == "" {
			return "", nil, false
		}
		return path, field, true
	case *types.Struct:
		for next := range t.Fields() {
			if next.Name() == "_" {
				continue
			}
			name := next.Name()
			if path != "" {
				name = path + "." + name
			}
			found, owner, ok := fieldPointer(next.Type(), name, next, seen)
			if ok {
				return found, owner, true
			}
		}
	case *types.Array:
		if t.Len() == 0 {
			return "", nil, false
		}
		return fieldPointer(t.Elem(), path+"[]", field, seen)
	case *types.Slice:
		return fieldPointer(t.Elem(), path+"[]", field, seen)
	case *types.Map:
		return fieldPointer(t.Elem(), path+"[]", field, seen)
	}
	return "", nil, false
}

func identityDiagnostic(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, stack []ast.Node, left, right types.Type) (analysis.Diagnostic, bool) {
	var (
		leftPointer, leftOK   = pointerOf(left)
		rightPointer, rightOK = pointerOf(right)
		bothPointers          = leftOK && rightOK
		bothUnsafe            = unsafePointer(left) && unsafePointer(right)
	)
	if !bothPointers && !bothUnsafe {
		return analysis.Diagnostic{}, false
	}
	if !assignable(left, right) {
		return analysis.Diagnostic{}, false
	}
	identical := types.Identical(left, right)
	var message string
	changes := true
	switch {
	case !identical:
		message = "reflect.DeepEqual on values of different types is always false; compare the pointers"
	case !bothUnsafe:
		if !reportablePointees(pass, leftPointer, rightPointer) {
			return analysis.Diagnostic{}, false
		}
		message = "reflect.DeepEqual on two pointers compares what they point to; compare the pointers"
	default:
		message = "reflect.DeepEqual on two pointers; compare the pointers"
		changes = false
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
	return diag, true
}

func oneLine(pass *analysis.Pass, call *ast.CallExpr) bool {
	var (
		start = pass.Fset.Position(call.Pos())
		end   = pass.Fset.Position(call.End())
	)
	return start.Line == end.Line
}

func assignable(left, right types.Type) bool {
	return types.AssignableTo(left, right) || types.AssignableTo(right, left)
}

func pointerOf(t types.Type) (*types.Pointer, bool) {
	ptr, ok := types.Unalias(t).Underlying().(*types.Pointer)
	return ptr, ok
}

func reportablePointees(pass *analysis.Pass, left, right *types.Pointer) bool {
	return plain(left.Elem()) && plain(right.Elem()) &&
		!hasEqual(pass, left.Elem()) && !hasEqual(pass, right.Elem())
}

func hasEqual(pass *analysis.Pass, t types.Type) bool {
	_, _, ok := equalMethod(pass, t)
	return ok
}

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

// needsHeaderParens reports whether an argument contains a composite literal
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
