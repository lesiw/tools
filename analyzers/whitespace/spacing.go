package whitespace

import (
	"go/ast"
	"go/token"
)

// commentSpacing puts a blank line before a doc comment inside a parenthesized
// block (const, var, type) or a struct field list. A comment that starts the
// block gets no blank line.
func commentSpacing(f *file) {
	var (
		tokFile = f.tokFile
		file    = f.syntax
	)
	ast.Inspect(file, func(n ast.Node) bool {
		var (
			ends []token.Pos
			docs []*ast.CommentGroup
		)
		switch n := n.(type) {
		case *ast.GenDecl:
			if !n.Lparen.IsValid() || len(n.Specs) < 2 {
				return true
			}
			for i, s := range n.Specs[1:] {
				var doc *ast.CommentGroup
				switch s := s.(type) {
				case *ast.ValueSpec:
					doc = s.Doc
				case *ast.TypeSpec:
					doc = s.Doc
				}
				ends = append(ends, n.Specs[i].End())
				docs = append(docs, doc)
			}
		case *ast.StructType:
			if n.Fields == nil || len(n.Fields.List) < 2 {
				return true
			}
			for i, f := range n.Fields.List[1:] {
				ends = append(ends, n.Fields.List[i].End())
				docs = append(docs, f.Doc)
			}
		case *ast.InterfaceType:
			if n.Methods == nil || len(n.Methods.List) < 2 {
				return true
			}
			for i, f := range n.Methods.List[1:] {
				ends = append(ends, n.Methods.List[i].End())
				docs = append(docs, f.Doc)
			}
		default:
			return true
		}
		spaceDocs(tokFile, file, ends, docs)
		return true
	})
}

// spaceDocs inserts a blank line before each doc comment in docs. A comment
// that already has one is left alone. ends[i] is the end of the element before
// docs[i].
func spaceDocs(tokFile *token.File, file *ast.File, ends []token.Pos, docs []*ast.CommentGroup) {
	for i, doc := range docs {
		if doc == nil || len(doc.List) == 0 {
			continue
		}
		if lineAt(tokFile, doc.Pos()) >= lineAt(tokFile, ends[i])+2 {
			continue // already has a blank line
		}
		addNewline(tokFile, pastTrailingComment(tokFile, file, ends[i]))
	}
}

// pastTrailingComment returns the end of a comment that trails pos on its
// line. It returns pos when no comment does. A line boundary at pos alone
// would push the comment onto the following line, away from the code it
// trails.
func pastTrailingComment(tokFile *token.File, file *ast.File, pos token.Pos) token.Pos {
	at := lineAt(tokFile, pos)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if c.Slash < pos || lineAt(tokFile, c.Slash) != at {
				continue
			}
			if c.End() > pos {
				pos = c.End()
			}
		}
	}
	return pos
}

// methodSpacing puts at least one blank line between a multi-line top-level
// declaration and each neighbor. Adjacent single-line declarations stay
// packed.
func methodSpacing(f *file) {
	var (
		tokFile = f.tokFile
		file    = f.syntax

		lastMulti bool
		lastEnd   token.Pos
	)
	// End is one past the declaration. A blank line set in the gap after
	// it would read as a second line, so multi measures to the last byte
	// the declaration occupies.
	for i, decl := range file.Decls {
		var (
			pos      = decl.Pos()
			prev     = lineAt(tokFile, lastEnd)
			multi    = lineAt(tokFile, pos) < lineAt(tokFile, decl.End()-1)
			adjacent = i > 0 && prev+1 == lineAt(tokFile, pos)
		)
		if (multi || lastMulti) && adjacent {
			addNewline(tokFile, pastTrailingComment(tokFile, file, lastEnd))
		}
		lastMulti = multi
		lastEnd = decl.End()
	}
}
