package main

import (
	"go/ast"
	"go/token"
)

// headerSpans returns the span of every if, for, range, and switch
// header: the keyword through the body's opening brace. Headers are
// single-line-or-nothing, so layout passes treat the span as
// atomic: never wrapped into, never expanded, collapsed when the
// whole header fits.
func headerSpans(file *ast.File) [][2]token.Pos {
	var spans [][2]token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		var lo, hi token.Pos
		switch node := n.(type) {
		case *ast.IfStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.ForStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.RangeStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.SwitchStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		case *ast.TypeSwitchStmt:
			lo, hi = node.Pos(), node.Body.Lbrace
		default:
			return true
		}
		spans = append(spans, [2]token.Pos{lo, hi})
		return true
	})
	return spans
}

// inHeader reports whether pos falls inside any header span.
func inHeader(spans [][2]token.Pos, pos token.Pos) bool {
	for _, s := range spans {
		if pos >= s[0] && pos <= s[1] {
			return true
		}
	}
	return false
}

// collapseHeaders merges every multi-line header that fits within
// the width limit back onto one line.
func collapseHeaders(tokFile *token.File, file *ast.File, snap *snapshot) {
	for _, s := range headerSpans(file) {
		if tokFile.Line(s[0]) == tokFile.Line(s[1]) {
			continue
		}
		if lineCommentWithin(file, s[0], s[1]) {
			continue
		}
		if w, ok := snap.mergedWidth(s[0], s[1]); !ok || w > cfg.Len {
			continue
		}
		mergeSpan(tokFile, s[0], s[1])
	}
}

// signatureSpans returns the span of every declared signature the
// width limit exempts: a function or method declaration from its
// keyword through its body's opening brace, and an interface method's
// whole field. The spans mirror the ones linelen skips.
//
// A function-type declaration is deliberately absent: linelen measures
// it like any other line, so pulling one onto a single line could put
// it over the limit with no exemption to land on.
func signatureSpans(file *ast.File) [][2]token.Pos {
	var spans [][2]token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			end := node.Type.End()
			if node.Body != nil {
				end = node.Body.Lbrace
			}
			spans = append(spans, [2]token.Pos{node.Pos(), end})
		case *ast.InterfaceType:
			if node.Methods == nil {
				return true
			}
			for _, field := range node.Methods.List {
				if _, ok := field.Type.(*ast.FuncType); !ok {
					continue
				}
				spans = append(spans, [2]token.Pos{field.Pos(), field.End()})
			}
		}
		return true
	})
	return spans
}

// collapseSignatures pulls every wrapped declared signature back onto
// one line. These are the lines the width limit exempts, and the
// exemption exists so a signature reads as one line, so the merge does
// not consult width at all. It withholds only where a comment inside
// the span would be swallowed.
func collapseSignatures(tokFile *token.File, file *ast.File) {
	for _, s := range signatureSpans(file) {
		if tokFile.Line(s[0]) == tokFile.Line(s[1]) {
			continue
		}
		if commentWithin(file, s[0], s[1]) {
			continue
		}
		mergeSpan(tokFile, s[0], s[1])
	}
}

// funcSignatures returns the *ast.FuncType nodes that are declared
// function, method, or function-type signatures — FuncDecls (with or
// without a receiver), interface methods (*ast.Field inside an
// InterfaceType.Methods.List whose Type is *ast.FuncType), and
// function-type declarations (TypeSpecs whose Type is *ast.FuncType).
// Anonymous FuncLits are excluded: their wrapped signatures are not exempt
// from collapse rules. Signatures that render awkwardly on pkgsite when
// wrapped stay as the author laid them out.
func funcSignatures(file *ast.File) []*ast.FuncType {
	var out []*ast.FuncType
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Type != nil {
				out = append(out, n.Type)
			}
		case *ast.TypeSpec:
			if ft, ok := n.Type.(*ast.FuncType); ok {
				out = append(out, ft)
			}
		case *ast.InterfaceType:
			if n.Methods == nil {
				return true
			}
			for _, f := range n.Methods.List {
				if ft, ok := f.Type.(*ast.FuncType); ok {
					out = append(out, ft)
				}
			}
		}
		return true
	})
	return out
}

// inFuncSignature reports whether pos falls inside any FuncType in sigs.
func inFuncSignature(sigs []*ast.FuncType, pos token.Pos) bool {
	for _, ft := range sigs {
		if pos >= ft.Pos() && pos <= ft.End() {
			return true
		}
	}
	return false
}
