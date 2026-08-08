package main

import (
	"go/ast"
	"go/token"
	"slices"
)

// expandOneLineBodies gives a block body its own lines when the line it
// shares with its header runs past the width limit. A body that fits
// stays where the author put it, and a body nested inside one that was
// just opened is measured against the line it lands on, so opening the
// outer body is enough when that is all the line needed.
func expandOneLineBodies(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) {
	var (
		placed []placement
		opened []*ast.BlockStmt
	)
	ast.Inspect(file, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		if body == nil || len(body.List) == 0 {
			return true
		}
		if !body.Lbrace.IsValid() || !body.Rbrace.IsValid() {
			return true
		}
		if tokFile.Line(body.Lbrace) != tokFile.Line(body.Rbrace) {
			return true
		}
		var (
			indent = snap.indent(body.Lbrace)
			line   = string(snap.text(snap.line(body.Lbrace)))
		)
		if p, ok := innermost(placed, body.Lbrace); ok {
			indent, line = p.indent, p.text
		}
		if width([]byte(line)) <= cfg.Len {
			return true
		}
		opened = append(opened, body)
		inner := append(slices.Clone(indent), '\t')
		for _, stmt := range body.List {
			s, err := printNode(fset, stmt)
			if err != nil {
				continue
			}
			placed = append(placed, placement{
				lo:     stmt.Pos(),
				hi:     stmt.End(),
				indent: inner,
				text:   string(inner) + s,
			})
		}
		return true
	})
	for _, body := range opened {
		for _, stmt := range body.List {
			addNewline(tokFile, stmt.Pos())
		}
		addNewline(tokFile, body.Rbrace)
	}
}
