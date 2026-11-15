// Package preferwritebyte provides an analysis.Analyzer that reports a
// one-element byte slice literal written to a writer, and that literal
// converted to a string.
//
// io.Writer's Write writes "len(p) bytes from p" to the underlying data
// stream, and io.StringWriter's WriteString writes the bytes of the string.
// The spec's conversions give a byte slice and a string the same bytes, so
// writing one byte through either builds a one-element slice or a one-byte
// string. io.ByteWriter's WriteByte takes the byte.
//
// The rule assumes Write, WriteString and WriteByte write the same bytes,
// which io.ByteWriter's doc does not require of an implementation.
//
// The README lists every exemption and every call reported without a fix.
//
// The rule is like go-critic's preferWriteByte, which reports a WriteRune
// call whose rune literal is one byte; this rule reports the Write and
// WriteString forms, and leaves WriteRune alone.
package preferwritebyte

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "preferwritebyte",
	Doc:  "reports a one-element byte slice literal written to a writer, or converted to a string",
	Run:  run,
}

var (
	byteSlice = types.NewSlice(types.Typ[types.Byte])
	errorType = types.Universe.Lookup("error").Type()

	writerSig       = newSignature(byteSlice, types.Typ[types.Int], errorType)
	stringWriterSig = newSignature(
		types.Typ[types.String],
		types.Typ[types.Int], errorType,
	)
	byteWriterSig = newSignature(types.Typ[types.Byte], errorType)
)

// newSignature returns the type of a method that takes one argument and
// returns results. It carries no receiver, because types.Identical ignores
// receivers when it compares two signatures.
func newSignature(arg types.Type, results ...types.Type) *types.Signature {
	var list []*types.Var
	for _, t := range results {
		list = append(list, types.NewVar(0, nil, "", t))
	}
	return types.NewSignatureType(nil, nil, nil,
		types.NewTuple(types.NewVar(0, nil, "", arg)),
		types.NewTuple(list...),
		false,
	)
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		c := &checker{
			pass:       pass,
			file:       file,
			statements: statements(file),
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				c.check(call)
			}
			return true
		})
	}
	return nil, nil
}

type checker struct {
	pass       *analysis.Pass
	file       *ast.File
	statements map[*ast.CallExpr]bool
}

// check reports one write of one byte, when the receiver can reach a WriteByte
// to call instead.
func (c *checker) check(call *ast.CallExpr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 {
		return
	}
	method, ok := c.pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return
	}
	sig, ok := method.Type().(*types.Signature)
	if !ok {
		return
	}
	var kind string
	var value ast.Expr
	switch method.Name() {
	case "Write":
		if !types.Identical(sig, writerSig) {
			return
		}
		kind = "slice"
		value, ok = sliceElement(c.pass, call.Args[0])
	case "WriteString":
		if !types.Identical(sig, stringWriterSig) {
			return
		}
		kind = "string"
		value, ok = stringElement(c.pass, call.Args[0])
	default:
		return
	}
	if !ok {
		return
	}
	writeByte, ok := c.writeByte(sel, method)
	if !ok {
		return
	}
	// Rewriting a call in the package that declares WriteByte could make
	// WriteByte call itself.
	if writeByte.Pkg() == c.pass.Pkg {
		return
	}
	if c.testFile() && c.testSubject(method) {
		// A test or an example of Write may be exercising it.
		return
	}
	diag := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "one-byte " + kind + " written to a writer; use WriteByte",
	}
	diag.SuggestedFixes = c.fix(call, value, sel.Sel)
	c.pass.Report(diag)
}

// sliceElement returns the single element of e, when e is a byte slice literal
// of one element and no key.
func sliceElement(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 1 {
		return nil, false
	}
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); keyed {
		// A keyed element supplies its index, so one element does not mean
		// one byte.
		return nil, false
	}
	t := pass.TypesInfo.TypeOf(lit)
	if t == nil {
		return nil, false
	}
	slice, ok := t.Underlying().(*types.Slice)
	if !ok || !types.Identical(slice.Elem(), types.Typ[types.Byte]) {
		return nil, false
	}
	return ast.Unparen(lit.Elts[0]), true
}

// stringElement returns the single element of the byte slice literal that e
// converts to a string.
func stringElement(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, false
	}
	if !types.Identical(tv.Type, types.Typ[types.String]) {
		return nil, false
	}
	return sliceElement(pass, call.Args[0])
}

// writeByte returns the receiver's WriteByte, when it has io.ByteWriter's
// signature, the same receiver form as method, and the same embedded fields.
func (c *checker) writeByte(sel *ast.SelectorExpr, method *types.Func) (*types.Func, bool) {
	recv := c.pass.TypesInfo.TypeOf(sel.X)
	if recv == nil {
		return nil, false
	}
	addressable := c.pass.TypesInfo.Types[sel.X].Addressable()
	_, methodPath, _ := types.LookupFieldOrMethod(
		recv, addressable,
		c.pass.Pkg, method.Name(),
	)
	obj, bytePath, _ := types.LookupFieldOrMethod(
		recv, addressable,
		c.pass.Pkg, "WriteByte",
	)
	other, ok := obj.(*types.Func)
	if !ok || !types.Identical(other.Type(), byteWriterSig) {
		return nil, false
	}
	methodRecv := method.Signature().Recv()
	otherRecv := other.Signature().Recv()
	_, methodPointer := methodRecv.Type().(*types.Pointer)
	_, otherPointer := otherRecv.Type().(*types.Pointer)
	if methodPointer != otherPointer {
		// A value receiver copies the receiver, so the two methods can write
		// to different places.
		return nil, false
	}
	if !samePath(methodPath, bytePath) {
		// The two methods may write to different places.
		return nil, false
	}
	return other, true
}

// samePath reports whether two lookup paths reach members of the same embedded
// field, so a Write the type declares does not match a WriteByte promoted from
// an embedded field. The last element of a path is the member's index in the
// type that declares it, so comparing everything before it compares the
// fields. An interface flattens its embedded interfaces, so its paths hold one
// element and its methods always match.
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

// fix returns the fix that drops the slice literal or string conversion around
// the byte and writes WriteByte in place of the method name, or nothing when
// the call is not the whole statement or a comment sits in the text the fix
// would delete.
func (c *checker) fix(call *ast.CallExpr, value ast.Expr, name *ast.Ident) []analysis.SuggestedFix {
	if !c.statements[call] {
		// WriteByte returns only an error, so a call whose count the code
		// keeps cannot be rewritten.
		return nil
	}
	var (
		arg   = call.Args[0]
		edits = []analysis.TextEdit{
			{Pos: arg.Pos(), End: value.Pos()},
			{Pos: value.End(), End: arg.End()},
		}
	)
	for _, edit := range edits {
		if commented(c.file.Comments, edit.Pos, edit.End) {
			return nil
		}
	}
	edits = append(edits, analysis.TextEdit{
		Pos:     name.Pos(),
		End:     name.End(),
		NewText: []byte("WriteByte"),
	})
	return []analysis.SuggestedFix{{
		Message:   "Call WriteByte",
		TextEdits: edits,
	}}
}

// statements returns the calls that stand alone in a statement, whose results
// nothing reads.
func statements(file *ast.File) map[*ast.CallExpr]bool {
	var (
		found  = make(map[*ast.CallExpr]bool)
		record = func(e ast.Expr) {
			if call, ok := ast.Unparen(e).(*ast.CallExpr); ok {
				found[call] = true
			}
		}
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ExprStmt:
			record(n.X)
		case *ast.DeferStmt:
			record(n.Call)
		case *ast.GoStmt:
			record(n.Call)
		}
		return true
	})
	return found
}

func (c *checker) testFile() bool {
	return strings.HasSuffix(c.pass.Fset.File(c.file.Pos()).Name(), "_test.go")
}

// testSubject reports whether method is declared by the package an external
// test package tests, whose path suffixes _test.
func (c *checker) testSubject(method *types.Func) bool {
	pkg := method.Pkg()
	if pkg == nil {
		return false
	}
	return pkg.Path() == strings.TrimSuffix(c.pass.Pkg.Path(), "_test")
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
