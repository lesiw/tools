package main

import (
	"go/ast"
	"go/token"
)

// commentSpacing ensures a blank line precedes any doc comment inside a
// parenthesized block (const, var, type) or struct field list, unless the
// comment starts the block.
func commentSpacing(tokFile *token.File, file *ast.File) {
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
		default:
			return true
		}
		spaceDocs(tokFile, file, ends, docs)
		return true
	})
}

// spaceDocs inserts a blank line before every doc comment in docs that
// does not already have one. ends[i] is the end of the element that
// precedes docs[i].
func spaceDocs(tokFile *token.File, file *ast.File, ends []token.Pos, docs []*ast.CommentGroup) {
	for i, doc := range docs {
		if doc == nil || len(doc.List) == 0 {
			continue
		}
		if tokFile.Line(doc.Pos()) >= tokFile.Line(ends[i])+2 {
			continue // already has a blank line
		}
		addNewline(tokFile, pastTrailingComment(tokFile, file, ends[i]))
	}
}

// pastTrailingComment returns the end of a comment trailing pos on its
// line, or pos when none does. A line boundary set at pos alone would
// push such a comment onto the following line, away from the code it
// trails.
func pastTrailingComment(tokFile *token.File, file *ast.File, pos token.Pos) token.Pos {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if c.Slash < pos || tokFile.Line(c.Slash) != tokFile.Line(pos) {
				continue
			}
			if c.End() > pos {
				pos = c.End()
			}
		}
	}
	return pos
}

// methodSpacing ensures a multi-line top-level declaration is
// separated from either neighbor by at least one blank line.
// Adjacent single-line declarations stay packed.
func methodSpacing(tokFile *token.File, file *ast.File) {
	var (
		lastMulti bool
		lastEnd   token.Pos
	)
	for i, decl := range file.Decls {
		var (
			pos      = decl.Pos()
			multi    = tokFile.Line(pos) < tokFile.Line(decl.End()-1)
			adjacent = i > 0 && tokFile.Line(lastEnd)+1 == tokFile.Line(pos)
		)
		if (multi || lastMulti) && adjacent {
			addNewline(tokFile, pastTrailingComment(tokFile, file, lastEnd))
		}
		lastMulti = multi
		lastEnd = decl.End()
	}
}
