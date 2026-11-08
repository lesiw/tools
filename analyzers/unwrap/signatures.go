package unwrap

import (
	"go/ast"
	"go/token"
)

// signature is one declared signature: the type that defines it and the span
// the width limit exempts. The span is invalid for a signature the limit
// measures like any other line.
type signature struct {
	typ  *ast.FuncType
	span [2]token.Pos
}

// signatures returns every declared signature in file — function and method
// declarations, interface methods, and function-type declarations. The exempt
// span mirrors the one linelen skips: a declaration from its keyword through
// its body's opening brace, and an interface method's whole field.
//
// A function-type declaration has no exempt span. linelen measures it like any
// other line, so pulling one onto a single line could put it over the limit
// with no exemption to land on.
func signatures(file *ast.File) (out []signature) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Type == nil {
				return true
			}
			end := node.Type.End()
			if node.Body != nil {
				end = node.Body.Lbrace
			}
			out = append(out, signature{
				typ:  node.Type,
				span: [2]token.Pos{node.Pos(), end},
			})
		case *ast.TypeSpec:
			if ft, ok := node.Type.(*ast.FuncType); ok {
				out = append(out, signature{typ: ft})
			}
		case *ast.InterfaceType:
			if node.Methods == nil {
				return true
			}
			for _, field := range node.Methods.List {
				if ft, ok := field.Type.(*ast.FuncType); ok {
					out = append(out, signature{
						typ:  ft,
						span: [2]token.Pos{field.Pos(), field.End()},
					})
				}
			}
		}
		return true
	})
	return out
}

// collapseSignatures pulls every wrapped declared signature back onto one
// line. The width limit exempts these lines, and the exemption exists so a
// signature reads as one line, so the merge does not consult width. It skips a
// signature with no exempt span — a function-type declaration, which the limit
// measures like any other line — and one whose merge would swallow a line
// comment.
func collapseSignatures(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
	)
	for _, sig := range signatures(file) {
		s := sig.span
		if !s[0].IsValid() || sameLine(tokFile, s[0], s[1]) {
			continue
		}
		if lineCommentWithin(file, s[0], s[1]) {
			continue
		}
		joinExempt(r, s[0], s[1])
	}
}

// inSignature reports whether pos falls inside a declared signature.
func inSignature(sigs []signature, pos token.Pos) bool {
	for _, sig := range sigs {
		if pos >= sig.typ.Pos() && pos <= sig.typ.End() {
			return true
		}
	}
	return false
}
