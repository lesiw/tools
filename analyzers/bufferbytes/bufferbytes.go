// Package bufferbytes provides an analysis.Analyzer that reports a
// bytes.Buffer converted to a string or a byte slice.
//
// bytes.Buffer's String returns the contents of the unread portion of the
// buffer as a string (https://pkg.go.dev/bytes#Buffer.String), and Bytes
// returns the same contents as a slice that aliases the buffer
// (https://pkg.go.dev/bytes#Buffer.Bytes). For a non-nil buffer,
// string(buf.Bytes()) and buf.String() return the same string.
//
// A conversion used as an index or map key is exempt. Map lookups and index
// assignments are the exemptions in S1030; delete, composite-literal keys,
// and conversions nested in a conversion or literal used as an index or key
// are also exempt. The exemption is lexical: it does not check the key type,
// element size, or compiler fast path. A conversion passed to the
// predeclared len is left to stringxbytes, as is one on the left of == or !=
// with the empty string literal on the right when len is predeclared. The
// bytes package is left alone, where a rewrite could call the method it is
// in. Its external tests are left alone, where a rewrite could make a test
// tautological.
//
// []byte(buf.String()) returns a copy, where Bytes returns a slice that
// aliases the buffer content, so the byte-slice form is reported without a
// fix. For a non-nil, non-empty buffer, bytes.Clone(buf.Bytes()) makes the
// same copy. A nil buffer makes Bytes panic where the conversion copies
// "<nil>", and when Bytes returns nil, Clone(nil) is nil where the conversion
// gives an empty slice. The string form is fixed when the receiver, or the
// embedded bytes.Buffer field, is a value, as in a bytes.Buffer value or a
// wrapper that embeds one, and the file is not a test file. A test may compare
// the conversion with the method, and the rewrite would make that comparison
// tautological.
//
// A pointer to bytes.Buffer is fixed by dereferencing it, as (*buf).String(),
// or directly when the expression cannot be nil, as with &v,
// new(bytes.Buffer), bytes.NewBuffer, or bytes.NewBufferString. An embedded
// *bytes.Buffer is fixed when each field on the path can be named, as
// (*w.Buffer).String(). Both fixes are withheld in a test file. An unexported
// field from another package is reported without a fix. A fix is withheld when
// an edit would delete a comment.
//
// The rule is inspired by staticcheck's S1030, which reports both forms. The
// byte-slice form is reported without a fix, and the map exemption is wider
// than the one in S1030. The README catalogs every check, exemption, and fix.
package bufferbytes

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "bufferbytes",
	Doc:  "reports a bytes.Buffer converted to a string or a byte slice",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if skipPackage(pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		skip := skipContexts(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call, skip)
			}
			return true
		})
	}
	return nil, nil
}

// skipPackage reports whether path is the bytes package or its external tests.
func skipPackage(path string) bool {
	return path == "bytes" || path == "bytes_test"
}

// check reports a conversion of a Buffer's Bytes to a string, or of its String
// to a byte slice, when the buffer has the method that returns the target
// directly.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, skip map[ast.Expr]struct{}) {
	conv, target, ok := converted(pass, call)
	if !ok {
		return
	}
	if _, skipped := skip[conv]; skipped {
		return
	}
	if isString(target) {
		inner, sel, ok := methodCall(pass, conv.Args[0], "Bytes")
		if !ok || !hasMethod(pass, sel, "String") {
			return
		}
		var fixes []analysis.SuggestedFix
		switch {
		case canFix(pass, file, sel):
			fixes = fix(file, conv, inner, sel, "String")
		case needsDeref(pass, sel) && !testFile(pass, file):
			fixes = fixDeref(file, conv, inner, sel, "String")
		case !testFile(pass, file):
			if fields, ok := embeddedPointerFields(pass, sel, "String"); ok {
				fixes = fixFieldDeref(file, conv, inner, sel, "String", fields)
			}
		}
		report(pass, conv, "conversion of Bytes to a string; String returns it for a non-nil buffer", fixes)
		return
	}
	if !byteSlice(target) {
		return
	}
	_, sel, ok := methodCall(pass, conv.Args[0], "String")
	if !ok || !hasMethod(pass, sel, "Bytes") {
		return
	}
	report(pass, conv, "conversion of String to a byte slice; Bytes aliases the buffer", nil)
}

// converted returns the conversion call e, its target type, and whether e is
// a one-argument conversion.
func converted(pass *analysis.Pass, e ast.Expr) (*ast.CallExpr, types.Type, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, nil, false
	}
	return call, tv.Type, true
}

// methodCall returns the call and selector for a call to the named
// bytes.Buffer method that takes no arguments.
func methodCall(pass *analysis.Pass, e ast.Expr, name string) (*ast.CallExpr, *ast.SelectorExpr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 || call.Ellipsis.IsValid() {
		return nil, nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return nil, nil, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || !bufferMethod(fn, name) {
		return nil, nil, false
	}
	return call, sel, true
}

// bufferMethod reports whether fn is the named method of bytes.Buffer.
func bufferMethod(fn *types.Func, name string) bool {
	if fn.Name() != name || fn.Pkg() == nil || fn.Pkg().Path() != "bytes" {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	ptr, ok := types.Unalias(sig.Recv().Type()).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Name() == "Buffer" &&
		obj.Pkg() != nil && obj.Pkg().Path() == "bytes"
}

// skipContexts returns the conversions stringxbytes reports in len and empty
// comparisons, and the conversions used as an index or map key.
func skipContexts(pass *analysis.Pass, file *ast.File) map[ast.Expr]struct{} {
	skip := make(map[ast.Expr]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IndexExpr:
			skipKey(pass, n.Index, skip)
		case *ast.CallExpr:
			switch {
			case isLen(pass, n) && len(n.Args) == 1:
				skip[ast.Unparen(n.Args[0])] = struct{}{}
			case isDelete(pass, n) && len(n.Args) == 2:
				skipKey(pass, n.Args[1], skip)
			}
		case *ast.CompositeLit:
			skipLiteralKeys(pass, n, skip)
		case *ast.BinaryExpr:
			if n.Op == token.EQL || n.Op == token.NEQ {
				if empty(pass, n.Y) && lenBuiltin(pass, file, n.X.Pos()) {
					skip[ast.Unparen(n.X)] = struct{}{}
				}
			}
		}
		return true
	})
	return skip
}

// skipLiteralKeys marks the conversions in a composite literal's keys.
func skipLiteralKeys(pass *analysis.Pass, lit *ast.CompositeLit, skip map[ast.Expr]struct{}) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			skipKey(pass, kv.Key, skip)
		}
	}
}

// skipKey marks the string conversions used as a key, including conversions
// nested in a conversion or literal that forms the key.
func skipKey(pass *analysis.Pass, e ast.Expr, skip map[ast.Expr]struct{}) {
	switch e := ast.Unparen(e).(type) {
	case *ast.CallExpr:
		tv, ok := pass.TypesInfo.Types[e.Fun]
		if !ok || !tv.IsType() {
			return
		}
		skip[e] = struct{}{}
		if len(e.Args) == 1 {
			skipKey(pass, e.Args[0], skip)
		}
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				skipKey(pass, kv.Value, skip)
				continue
			}
			skipKey(pass, elt, skip)
		}
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			skipKey(pass, e.X, skip)
		}
	}
}

// isLen reports whether call calls the predeclared len.
func isLen(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("len")
}

// isDelete reports whether call calls the predeclared delete.
func isDelete(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("delete")
}

// empty reports whether e is the empty string literal.
func empty(pass *analysis.Pass, e ast.Expr) bool {
	lit, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	tv, ok := pass.TypesInfo.Types[lit]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return false
	}
	return constant.StringVal(tv.Value) == ""
}

// lenBuiltin reports whether len names the predeclared function at pos. A
// shadowed len means stringxbytes will not report, so this check does not skip
// the conversion.
func lenBuiltin(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	root := pass.TypesInfo.Scopes[file]
	if root == nil {
		return false
	}
	_, obj := root.Innermost(pos).LookupParent("len", pos)
	return obj == types.Universe.Lookup("len")
}

// report emits the diagnostic for a conversion, with the fixes it may carry.
func report(pass *analysis.Pass, conv *ast.CallExpr, message string, fixes []analysis.SuggestedFix) {
	pass.Report(analysis.Diagnostic{
		Pos:            conv.Pos(),
		End:            conv.End(),
		Message:        message,
		SuggestedFixes: fixes,
	})
}

// hasMethod reports whether name resolves to the named bytes.Buffer method on
// the receiver. A wrapper method of that name hides the bytes.Buffer method.
func hasMethod(pass *analysis.Pass, sel *ast.SelectorExpr, name string) bool {
	recv := pass.TypesInfo.TypeOf(sel.X)
	if recv == nil {
		return false
	}
	addressable := pass.TypesInfo.Types[sel.X].Addressable()
	obj, _, _ := types.LookupFieldOrMethod(recv, addressable, pass.Pkg, name)
	fn, ok := obj.(*types.Func)
	return ok && bufferMethod(fn, name)
}

// canFix reports whether the receiver or the field holding the buffer is a
// value, or the receiver is a non-nil pointer expression, so the rewrite does
// not turn a nil buffer's panic into String's "<nil>".
func canFix(pass *analysis.Pass, file *ast.File, sel *ast.SelectorExpr) bool {
	if testFile(pass, file) {
		return false
	}
	recv := pass.TypesInfo.TypeOf(sel.X)
	if recv == nil {
		return false
	}
	addressable := pass.TypesInfo.Types[sel.X].Addressable()
	obj, path, _ := types.LookupFieldOrMethod(
		recv, addressable,
		pass.Pkg, "String",
	)
	fn, ok := obj.(*types.Func)
	if !ok || !bufferMethod(fn, "String") {
		return false
	}
	t := recv
	for _, index := range path[:len(path)-1] {
		t = deref(t)
		st, ok := structType(t)
		if !ok || index < 0 || index >= st.NumFields() {
			return false
		}
		t = st.Field(index).Type()
	}
	if !isPointer(t) {
		return true
	}
	return len(path) == 1 && nonNil(pass, sel.X)
}

// needsDeref reports whether the receiver is a *bytes.Buffer, whose fix
// dereferences it to preserve the nil panic.
func needsDeref(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	ptr, ok := types.Unalias(pass.TypesInfo.TypeOf(sel.X)).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Name() == "Buffer" &&
		obj.Pkg() != nil && obj.Pkg().Path() == "bytes"
}

func testFile(pass *analysis.Pass, file *ast.File) bool {
	return strings.HasSuffix(pass.Fset.File(file.Pos()).Name(), "_test.go")
}

// nonNil reports whether e yields a non-nil pointer or panics before a method
// call, so a nil buffer cannot turn the panic into String's "<nil>".
func nonNil(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.UnaryExpr:
		return e.Op == token.AND
	case *ast.CallExpr:
		if id, ok := ast.Unparen(e.Fun).(*ast.Ident); ok {
			return pass.TypesInfo.Uses[id] == types.Universe.Lookup("new")
		}
		sel, ok := ast.Unparen(e.Fun).(*ast.SelectorExpr)
		if !ok {
			return false
		}
		fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "bytes" {
			return false
		}
		return fn.Name() == "NewBuffer" || fn.Name() == "NewBufferString"
	}
	return false
}

// structType returns t's underlying struct, if it has one.
func structType(t types.Type) (*types.Struct, bool) {
	st, ok := types.Unalias(t).Underlying().(*types.Struct)
	return st, ok
}

func deref(t types.Type) types.Type {
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

func isPointer(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.Pointer)
	return ok
}

// fix returns the edits that drop the conversion and rename the method, or
// nothing when an edit would delete a comment.
func fix(file *ast.File, conv, inner *ast.CallExpr, sel *ast.SelectorExpr, name string) []analysis.SuggestedFix {
	edits := []analysis.TextEdit{
		{Pos: conv.Pos(), End: sel.Pos()},
		{Pos: sel.Sel.Pos(), End: sel.Sel.End(), NewText: []byte(name)},
		{Pos: inner.End(), End: conv.End()},
	}
	for _, edit := range edits {
		if edit.Pos == edit.End {
			continue
		}
		if commented(file.Comments, edit.Pos, edit.End) {
			return nil
		}
	}
	return []analysis.SuggestedFix{{
		Message:   "Call " + name,
		TextEdits: edits,
	}}
}

// fixDeref returns the edits that drop the conversion, dereference the
// receiver, and rename the method, or nothing when an edit would delete a
// comment.
func fixDeref(file *ast.File, conv, inner *ast.CallExpr, sel *ast.SelectorExpr, name string) []analysis.SuggestedFix {
	edits := []analysis.TextEdit{
		{Pos: conv.Pos(), End: sel.Pos(), NewText: []byte("(*")},
		{Pos: sel.X.End(), End: sel.X.End(), NewText: []byte(")")},
		{Pos: sel.Sel.Pos(), End: sel.Sel.End(), NewText: []byte(name)},
		{Pos: inner.End(), End: conv.End()},
	}
	for _, edit := range edits {
		if edit.Pos == edit.End {
			continue
		}
		if commented(file.Comments, edit.Pos, edit.End) {
			return nil
		}
	}
	return []analysis.SuggestedFix{{
		Message:   "Call " + name,
		TextEdits: edits,
	}}
}

// embeddedPointerFields returns the field names from the receiver to an
// embedded *bytes.Buffer, when every field can be named.
func embeddedPointerFields(pass *analysis.Pass, sel *ast.SelectorExpr, name string) ([]string, bool) {
	recv := pass.TypesInfo.TypeOf(sel.X)
	if recv == nil {
		return nil, false
	}
	addressable := pass.TypesInfo.Types[sel.X].Addressable()
	obj, path, _ := types.LookupFieldOrMethod(
		recv, addressable,
		pass.Pkg, name,
	)
	fn, ok := obj.(*types.Func)
	if !ok || !bufferMethod(fn, name) || len(path) == 0 {
		return nil, false
	}
	t := recv
	var names []string
	for _, index := range path[:len(path)-1] {
		t = deref(t)
		st, ok := structType(t)
		if !ok || index < 0 || index >= st.NumFields() {
			return nil, false
		}
		field := st.Field(index)
		if !field.Exported() && field.Pkg() != nil && field.Pkg() != pass.Pkg {
			return nil, false
		}
		names = append(names, field.Name())
		t = field.Type()
	}
	if len(names) == 0 || !isPointer(t) {
		return nil, false
	}
	return names, true
}

// fixFieldDeref returns the edits that drop the conversion, dereference an
// embedded field, and rename the method, or nothing when an edit would delete
// a comment.
func fixFieldDeref(file *ast.File, conv, inner *ast.CallExpr, sel *ast.SelectorExpr, name string, fields []string) []analysis.SuggestedFix {
	var b strings.Builder
	for _, name := range fields {
		b.WriteString(".")
		b.WriteString(name)
	}
	b.WriteString(")")
	edits := []analysis.TextEdit{
		{Pos: conv.Pos(), End: sel.Pos(), NewText: []byte("(*")},
		{Pos: sel.X.End(), End: sel.X.End(), NewText: []byte(b.String())},
		{Pos: sel.Sel.Pos(), End: sel.Sel.End(), NewText: []byte(name)},
		{Pos: inner.End(), End: conv.End()},
	}
	for _, edit := range edits {
		if edit.Pos == edit.End {
			continue
		}
		if commented(file.Comments, edit.Pos, edit.End) {
			return nil
		}
	}
	return []analysis.SuggestedFix{{
		Message:   "Call " + name,
		TextEdits: edits,
	}}
}

// isString reports whether t is the predeclared string type or an alias of it.
func isString(t types.Type) bool {
	return types.Identical(types.Unalias(t), types.Typ[types.String])
}

// byteSlice reports whether t is []byte or an alias of it.
func byteSlice(t types.Type) bool {
	slice, ok := types.Unalias(t).(*types.Slice)
	if !ok {
		return false
	}
	return types.Identical(types.Unalias(slice.Elem()), types.Typ[types.Byte])
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
