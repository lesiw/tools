// Package preferstringwriter provides an analysis.Analyzer that reports a
// converted string written to a writer.
//
// io.WriteString's doc invokes a StringWriter's WriteString directly and calls
// Write "exactly once" otherwise. The rule assumes the two write the same
// bytes. Where one type declares both, w.Write([]byte(s)) copies the string
// into a byte slice first, and w.WriteString(s) writes it straight to the
// writer.
//
// Such a call is reported where a type outside the caller's package declares
// both methods. The README lists every exemption and the fix.
//
// The rule is adapted from go-critic's preferStringWriter; this one leaves
// io.WriteString(w, s) alone.
package preferstringwriter

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "preferstringwriter",
	Doc:  "reports a converted string written to a writer",
	Run:  run,
}

var (
	byteSlice       = types.NewSlice(types.Typ[types.Byte])
	stringType      = types.Typ[types.String]
	untypedString   = types.Typ[types.UntypedString]
	writerSig       = newSignature(byteSlice)
	stringWriterSig = newSignature(stringType)
)

// newSignature returns the type of a method that takes one argument and
// returns a count and an error. It carries no receiver, because
// types.Identical ignores receivers when it compares two signatures.
func newSignature(arg types.Type) *types.Signature {
	return types.NewSignatureType(nil, nil, nil,
		types.NewTuple(types.NewVar(0, nil, "", arg)),
		types.NewTuple(
			types.NewVar(0, nil, "", types.Typ[types.Int]),
			types.NewVar(0, nil, "", types.Universe.Lookup("error").Type()),
		),
		false,
	)
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports one write of a converted string, when the receiver can reach a
// WriteString to call instead.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Write" || len(call.Args) != 1 {
		return
	}
	conv, assignable, ok := conversion(pass, call.Args[0])
	if !ok {
		return
	}
	recv, ok := pass.TypesInfo.Types[sel.X]
	if !ok || !paired(pass, recv) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        "converted string written to a writer; use WriteString",
		SuggestedFixes: fix(file, conv, assignable, sel.Sel),
	})
}

// conversion returns the call that converts a string to a byte slice, and
// whether the operand is assignable to string.
func conversion(pass *analysis.Pass, e ast.Expr) (conv *ast.CallExpr, assignable, ok bool) {
	conv, ok = ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, false, false
	}
	tv := pass.TypesInfo.Types[conv.Fun]
	if !tv.IsType() || !types.Identical(tv.Type.Underlying(), byteSlice) {
		return nil, false, false
	}
	operand := pass.TypesInfo.TypeOf(conv.Args[0])
	if operand == nil {
		return nil, false, false
	}
	// The operand is a string or a type parameter whose core type is one,
	// which is the set an untyped string constant is assignable to. Anything
	// else cannot be passed to WriteString.
	if !types.AssignableTo(untypedString, operand) {
		return nil, false, false
	}
	return conv, types.AssignableTo(operand, stringType), true
}

// paired reports whether one type outside the caller's package declares both
// methods, with the signatures of io.Writer.Write and
// io.StringWriter.WriteString.
func paired(pass *analysis.Pass, recv types.TypeAndValue) bool {
	var (
		write       = declarer(recv, "Write", writerSig)
		writeString = declarer(recv, "WriteString", stringWriterSig)
	)
	if write == nil || !types.Identical(write, writeString) {
		return false
	}
	// The rewrite can land back inside a WriteString this package declares.
	named, ok := write.(*types.Named)
	return ok && named.Obj().Pkg() != pass.Pkg
}

// declarer returns the type that declares the method a value of type v
// reaches, when the method's signature is sig. A value whose type lacks the
// method can still call it through its pointer, which Go permits wherever the
// value is addressable.
func declarer(v types.TypeAndValue, name string, sig *types.Signature) types.Type {
	obj, _, _ := types.LookupFieldOrMethod(v.Type, v.Addressable(), nil, name)
	if fn, ok := obj.(*types.Func); ok && types.Identical(fn.Type(), sig) {
		recv := types.Unalias(fn.Type().(*types.Signature).Recv().Type())
		if ptr, ok := recv.(*types.Pointer); ok {
			return ptr.Elem()
		}
		return recv
	}
	return nil
}

// fix returns the edits that drop the conversion and name WriteString, or nil
// when a comment sits in the text they delete.
func fix(file *ast.File, conv *ast.CallExpr, assignable bool, name *ast.Ident) []analysis.SuggestedFix {
	var (
		operand = ast.Unparen(conv.Args[0])
		edits   = []analysis.TextEdit{
			{Pos: conv.Pos(), End: operand.Pos()},
			{Pos: operand.End(), End: conv.End()},
		}
	)
	if !assignable {
		edits[0].NewText = []byte("string(")
		edits[1].NewText = []byte(")")
	}
	for _, edit := range edits {
		if commented(file.Comments, edit.Pos, edit.End) {
			return nil
		}
	}
	edits = append(edits, analysis.TextEdit{
		Pos:     name.Pos(),
		End:     name.End(),
		NewText: []byte("WriteString"),
	})
	return []analysis.SuggestedFix{{
		Message:   "Call WriteString",
		TextEdits: edits,
	}}
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
