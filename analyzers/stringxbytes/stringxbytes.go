// Package stringxbytes provides an analysis.Analyzer that reports a conversion
// between string and []byte that nothing needs.
//
// The spec gives copy and append a string form
// (https://go.dev/ref/spec#Appending_and_copying_slices), and io.WriteString
// calls WriteString in place of Write, so a conversion handed to copy, append,
// or a Write whose receiver has a WriteString is a copy nothing reads. A
// conversion that only feeds len or an empty-string test is unnecessary too,
// since it keeps the number of bytes.
//
// copy, append, len, and a comparison with an empty string drop the
// conversion, and Write becomes WriteString. A fix is withheld when an edit
// would delete a comment.
//
// The README catalogs every check, exemption, and fix.
//
// The rule is like go-critic's stringXbytes and preferStringWriter.
package stringxbytes

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "stringxbytes",
	Doc:  "reports a conversion between string and []byte that nothing needs",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Walk(&visitor{pass: pass, file: file}, file)
	}
	return nil, nil
}

// visitor walks one file, remembering the function that holds each node so a
// call inside a method can recognize the method it is in.
type visitor struct {
	pass      *analysis.Pass
	file      *ast.File
	enclosing *types.Func
}

func (v *visitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	var (
		diag analysis.Diagnostic
		ok   bool
	)
	switch n := n.(type) {
	case *ast.FuncDecl:
		fn, _ := v.pass.TypesInfo.Defs[n.Name].(*types.Func)
		return &visitor{pass: v.pass, file: v.file, enclosing: fn}
	case *ast.CallExpr:
		diag, ok = v.callDiag(n)
	case *ast.BinaryExpr:
		diag, ok = v.compareDiag(n)
	}
	if ok {
		v.pass.Report(diag)
	}
	return v
}

// callDiag examines one call: a builtin that accepts the value the conversion
// came from, a method with a string sibling, or len of a conversion.
func (v *visitor) callDiag(call *ast.CallExpr) (analysis.Diagnostic, bool) {
	if name, ok := builtin(v.pass, call); ok {
		switch name {
		case "copy", "append":
			return v.sliceDiag(call, name)
		case "len":
			return v.lenDiag(call)
		}
		return analysis.Diagnostic{}, false
	}
	return v.siblingDiag(call)
}

// sliceDiag reports the conversion in copy(dst, []byte(s)) or in append(dst,
// []byte(s)...). Both special forms read a string source directly, so the
// conversion only copies bytes the call reads again.
func (v *visitor) sliceDiag(call *ast.CallExpr, name string) (analysis.Diagnostic, bool) {
	spread := name == "append"
	if call.Ellipsis.IsValid() != spread {
		return analysis.Diagnostic{}, false
	}
	conv, target, ok := converted(v.pass, call.Args[1])
	if !ok || !exactBytes(target) {
		return analysis.Diagnostic{}, false
	}
	if src := conv.Args[0]; !isString(v.pass.TypesInfo.TypeOf(src)) {
		return analysis.Diagnostic{}, false
	}
	message := "conversion only to " + name + "; " + name + " takes a string"
	return v.dropDiag(call.Args[1], conv.Args[0], message), true
}

// lenDiag reports the conversion in len(string(b)), whose length is len(b).
func (v *visitor) lenDiag(call *ast.CallExpr) (analysis.Diagnostic, bool) {
	conv, target, ok := converted(v.pass, call.Args[0])
	if !ok {
		return analysis.Diagnostic{}, false
	}
	src := conv.Args[0]
	if !sameLength(v.pass.TypesInfo.TypeOf(src), target) {
		return analysis.Diagnostic{}, false
	}
	message := "conversion only to take its length; len reads it directly"
	return v.dropDiag(call.Args[0], src, message), true
}

// sameLength reports whether converting between from and to keeps the number
// of bytes, which holds with a string on one side and a byte slice on the
// other.
func sameLength(from, to types.Type) bool {
	if isString(from) {
		return isByteSlice(to)
	}
	return isByteSlice(from) && isString(to)
}

// compareDiag reports a converted byte slice compared with the empty string.
func (v *visitor) compareDiag(bin *ast.BinaryExpr) (analysis.Diagnostic, bool) {
	if bin.Op != token.EQL && bin.Op != token.NEQ {
		return analysis.Diagnostic{}, false
	}
	conv, src, ok := bytesToText(v.pass, bin.X)
	if !ok || !empty(v.pass, bin.Y) {
		return analysis.Diagnostic{}, false
	}
	if !lenBuiltin(v.pass, v.file, conv.Pos()) {
		return analysis.Diagnostic{}, false
	}
	// bin.X may wrap the conversion in parentheses, so the tail edit replaces
	// everything after the operand with one closing parenthesis.
	edits := []analysis.TextEdit{
		{Pos: bin.X.Pos(), End: src.Pos(), NewText: []byte("len(")},
		{Pos: src.End(), End: bin.X.End(), NewText: []byte(")")},
		{Pos: bin.Y.Pos(), End: bin.Y.End(), NewText: []byte("0")},
	}
	return analysis.Diagnostic{
		Pos:            bin.Pos(),
		End:            bin.End(),
		Message:        "conversion only to compare with an empty string; use len",
		SuggestedFixes: v.fix("Compare the length", edits...),
	}, true
}

// siblingDiag reports a call to Write that hands it a string conversion, when
// the receiver has a WriteString that takes the string and returns the same
// results.
func (v *visitor) siblingDiag(call *ast.CallExpr) (analysis.Diagnostic, bool) {
	sel, method, sig, ok := methodCall(v.pass, call)
	if !ok || method.Name() != "Write" || len(call.Args) != 1 {
		return analysis.Diagnostic{}, false
	}
	if v.testFile() && v.testSubject(method) {
		// A test or an example of Write may be exercising it, and the fix
		// would rename the call to WriteString.
		return analysis.Diagnostic{}, false
	}
	other, ok := v.writeString(sel)
	if !ok || !agrees(sig, other) {
		return analysis.Diagnostic{}, false
	}
	// A method of the receiver's own type may be one WriteString reaches, so
	// the rename could make the call recursive.
	if sameReceiver(v.enclosing, other) {
		return analysis.Diagnostic{}, false
	}
	src, ok := stringArg(v.pass, call.Args[0])
	if !ok {
		return analysis.Diagnostic{}, false
	}
	return v.renameDiag(sel, call.Args[0], src, other), true
}

// writeString returns the receiver's WriteString, if it has one.
func (v *visitor) writeString(sel *ast.SelectorExpr) (*types.Func, bool) {
	var (
		recv        = v.pass.TypesInfo.TypeOf(sel.X)
		addressable = v.pass.TypesInfo.Types[sel.X].Addressable()
	)
	// The two paths say where each method sits, for samePath to compare.
	_, write, _ := types.LookupFieldOrMethod(
		recv, addressable,
		v.pass.Pkg, "Write",
	)
	obj, writeString, _ := types.LookupFieldOrMethod(
		recv, addressable,
		v.pass.Pkg, "WriteString",
	)
	other, ok := obj.(*types.Func)
	if !ok || !samePath(write, writeString) {
		return nil, false
	}
	return other, true
}

// samePath reports whether two lookup paths run through the same embedded
// fields, so a type's own Write does not pair with a WriteString promoted from
// an embedded field. An interface flattens its embedded interfaces, so its
// paths each hold one element and its methods always agree.
func samePath(a, b []int) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	for i := range a[:len(a)-1] {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameReceiver reports whether method and other belong to the same type, so a
// call to other could reach method.
func sameReceiver(method, other *types.Func) bool {
	if method == nil {
		return false
	}
	a, b := method.Signature().Recv(), other.Signature().Recv()
	if a == nil || b == nil {
		return false
	}
	return types.Identical(underPointer(a.Type()), underPointer(b.Type()))
}

// underPointer returns t without the pointer it may be behind.
func underPointer(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// isPointer reports whether t is a pointer, the receiver form the pair has to
// agree on.
func isPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// agrees reports whether other, a WriteString, takes the string where Write
// takes a byte slice and returns what Write returns.
func agrees(sig *types.Signature, other *types.Func) bool {
	otherSig, ok := other.Type().(*types.Signature)
	if !ok {
		return false
	}
	params, otherParams := sig.Params(), otherSig.Params()
	if params.Len() != 1 || otherParams.Len() != 1 {
		return false
	}
	if !isByteSlice(params.At(0).Type()) {
		return false
	}
	if !types.Identical(otherParams.At(0).Type(), types.Typ[types.String]) {
		return false
	}
	if isPointer(sig.Recv().Type()) != isPointer(otherSig.Recv().Type()) {
		return false
	}
	return types.Identical(sig.Results(), otherSig.Results())
}

// renameDiag returns the report for a call, with the fix that renames it to
// other and drops the conversion.
func (v *visitor) renameDiag(sel *ast.SelectorExpr, arg, src ast.Expr, other *types.Func) analysis.Diagnostic {
	var (
		diag = analysis.Diagnostic{
			Pos: arg.Pos(),
			End: arg.End(),
			Message: "conversion only to call " + sel.Sel.Name + "; " +
				other.Name() + " takes a string",
		}
		edits = append(unconvert(arg, src), rename(sel, other.Name()))
	)
	diag.SuggestedFixes = v.fix("Call "+other.Name(), edits...)
	return diag
}

// dropDiag returns the report for a conversion nothing needs, with the fix
// that deletes it when no comment sits in the way.
func (v *visitor) dropDiag(arg, src ast.Expr, message string) analysis.Diagnostic {
	var (
		diag = analysis.Diagnostic{
			Pos:     arg.Pos(),
			End:     arg.End(),
			Message: message,
		}
		edits = unconvert(arg, src)
	)
	diag.SuggestedFixes = v.fix("Drop the conversion", edits...)
	return diag
}

// fix returns the fix that applies edits, or nothing when one of them would
// delete a comment.
func (v *visitor) fix(message string, edits ...analysis.TextEdit) []analysis.SuggestedFix {
	for _, e := range edits {
		if commented(v.file.Comments, e.Pos, e.End) {
			return nil
		}
	}
	return []analysis.SuggestedFix{{Message: message, TextEdits: edits}}
}

// testFile reports whether the visitor's file is a test file.
func (v *visitor) testFile() bool {
	return strings.HasSuffix(v.pass.Fset.File(v.file.Pos()).Name(), "_test.go")
}

// testSubject reports whether method belongs to the package the test file
// exercises. An external test package names that package by adding a _test
// suffix to its own path.
func (v *visitor) testSubject(method *types.Func) bool {
	pkg := method.Pkg()
	if pkg == nil {
		return false
	}
	subject := strings.TrimSuffix(v.pass.Pkg.Path(), "_test")
	return pkg.Path() == subject || pkg.Path() == v.pass.Pkg.Path()
}

// methodCall returns the selector, the method and the signature when call
// calls a non-variadic method.
func methodCall(pass *analysis.Pass, call *ast.CallExpr) (*ast.SelectorExpr, *types.Func, *types.Signature, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil, nil, false
	}
	method, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil, nil, nil, false
	}
	sig, ok := method.Type().(*types.Signature)
	if !ok || sig.Variadic() {
		return nil, nil, nil, false
	}
	return sel, method, sig, true
}

// converted returns the conversion expression e, its target type, and whether
// e is a conversion at all.
func converted(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, types.Type, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, nil, false
	}
	return call, tv.Type, true
}

// stringArg returns the operand of e when e is a conversion whose operand can
// go where a string is wanted. A named string type needs a conversion to reach
// one, so it is not.
func stringArg(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	conv, _, ok := converted(pass, e)
	if !ok {
		return nil, false
	}
	src := conv.Args[0]
	t := pass.TypesInfo.TypeOf(src)
	if t == nil || !types.AssignableTo(t, types.Typ[types.String]) {
		return nil, false
	}
	return src, true
}

// bytesToText returns the conversion and the byte slice operand when e is a
// conversion of a byte slice to a string.
func bytesToText(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, ast.Expr, bool) {
	conv, target, ok := converted(pass, e)
	if !ok || !isString(target) {
		return nil, nil, false
	}
	src := conv.Args[0]
	if !isByteSlice(pass.TypesInfo.TypeOf(src)) {
		return nil, nil, false
	}
	return conv, src, true
}

// unconvert returns the edits that delete a conversion's parentheses.
func unconvert(arg, src ast.Expr) []analysis.TextEdit {
	return []analysis.TextEdit{
		{Pos: arg.Pos(), End: src.Pos()},
		{Pos: src.End(), End: arg.End()},
	}
}

// rename returns the edit that gives a method call another name.
func rename(sel *ast.SelectorExpr, name string) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     sel.Sel.Pos(),
		End:     sel.Sel.End(),
		NewText: []byte(name),
	}
}

// empty reports whether e is the empty string literal.
func empty(pass *analysis.Pass, e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	tv, ok := pass.TypesInfo.Types[lit]
	return ok && tv.Value != nil && constant.StringVal(tv.Value) == ""
}

// commented reports whether a comment sits in the span a fix would delete.
func commented(comments []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}

// builtin returns the name of the predeclared function a call names.
func builtin(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return "", false
	}
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok {
		return "", false
	}
	return fn.Name(), true
}

// lenBuiltin reports whether len names the predeclared function at pos. A
// shadowed len would take the call the fix writes.
func lenBuiltin(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	root := pass.TypesInfo.Scopes[file]
	if root == nil {
		return false
	}
	_, obj := root.Innermost(pos).LookupParent("len", pos)
	return obj == types.Universe.Lookup("len")
}

// isString reports whether t is a string or has a string underlying type.
func isString(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// isByteSlice reports whether t is a slice whose elements are bytes.
func isByteSlice(t types.Type) bool {
	slice, ok := sliceOf(t)
	if !ok {
		return false
	}
	elem, ok := slice.Elem().Underlying().(*types.Basic)
	return ok && elem.Kind() == types.Byte
}

// exactBytes reports whether t is a slice of the predeclared byte itself, the
// element type copy's string form requires.
func exactBytes(t types.Type) bool {
	slice, ok := sliceOf(t)
	return ok && types.Identical(slice.Elem(), types.Typ[types.Byte])
}

// sliceOf returns t's underlying slice, if it has one.
func sliceOf(t types.Type) (*types.Slice, bool) {
	if t == nil {
		return nil, false
	}
	slice, ok := types.Unalias(t).Underlying().(*types.Slice)
	return slice, ok
}
