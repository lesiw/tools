// Package signature finds the function signatures in a file.
//
// A function type is a signature wherever it appears, and a signature stays on
// one line: the width rule does not count one, the join rule pulls a wrapped
// one onto one line, and the bracket rule leaves the lists inside one alone.
// The three agree because they read the set from here.
package signature

import (
	"go/ast"
	"go/token"
)

// Span is one signature: the function type that defines it, the first byte of
// its declaration, and the body that follows where a declaration or a literal
// carries one.
type Span struct {
	Type *ast.FuncType
	from token.Pos
	body *ast.BlockStmt
}

// Pos returns the first byte of the signature. An interface method begins at
// its name, because a function type has no name of its own to hold.
func (s Span) Pos() token.Pos { return s.from }

// End returns the last byte of the signature.
func (s Span) End() token.Pos { return s.Type.End() }

// BodyStart returns the position of the body's first statement. It returns
// NoPos for a type that carries no body, and for a body without statements.
func (s Span) BodyStart() token.Pos {
	if s.body == nil || len(s.body.List) == 0 {
		return token.NoPos
	}
	return s.body.List[0].Pos()
}

// Spans returns every signature in file. A function type is one wherever it
// appears: a declaration, a method, an interface method, a literal, a var or
// type declared with one, a struct field, a parameter, a result, or the value
// type of a map.
func Spans(file *ast.File) (spans []Span) {
	var (
		bodies = make(map[*ast.FuncType]*ast.BlockStmt)
		names  = make(map[*ast.FuncType]token.Pos)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			bodies[node.Type] = node.Body
		case *ast.FuncLit:
			bodies[node.Type] = node.Body
		case *ast.InterfaceType:
			if node.Methods == nil {
				return true
			}
			for _, field := range node.Methods.List {
				if ft, ok := field.Type.(*ast.FuncType); ok {
					names[ft] = field.Pos()
				}
			}
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		if ft, ok := n.(*ast.FuncType); ok {
			from := ft.Pos()
			if name, ok := names[ft]; ok {
				from = name
			}
			spans = append(spans, Span{Type: ft, from: from, body: bodies[ft]})
		}
		return true
	})
	return spans
}

// In reports whether pos falls inside the type of any span.
func In(spans []Span, pos token.Pos) bool {
	for _, s := range spans {
		if pos >= s.Pos() && pos <= s.End() {
			return true
		}
	}
	return false
}
