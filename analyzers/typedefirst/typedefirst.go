// Package typedefirst provides an analysis.Analyzer that reports a type
// declared after the methods on it.
//
// The compiler accepts either order, since a top-level type lives in the
// package block. A reader meets a method before the declaration of the type it
// is declared on.
//
// The fix moves the declaration, with its comment block, above the earliest
// method on a type it declares. It is withheld when the move would put the
// declaration above a name it mentions, and when the file changed since it was
// parsed, and the diagnostic is reported without one. The README catalogs
// every exemption.
//
// The rule is adapted from go-critic's typeDefFirst.
package typedefirst

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "typedefirst",
	Doc:  "reports a type declared after the methods on it",
	Run:  run,
}

// decl is a type declaration, the type in it to report, and the method that
// precedes it.
type decl struct {
	gen  *ast.GenDecl
	spec *ast.TypeSpec
	fn   *ast.FuncDecl
}

// source is one file of a package.
type source struct {
	*token.File
	pass   *analysis.Pass
	syntax *ast.File
	src    []byte
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tf := pass.Fset.File(file.FileStart)
		src, err := pass.ReadFile(tf.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", tf.Name(), err)
		}
		s := source{File: tf, pass: pass, syntax: file, src: src}
		s.check()
	}
	return nil, nil
}

// check reports a type whose first method is declared above it.
func (s source) check() {
	methods := make(map[string]*ast.FuncDecl)
	for _, node := range s.syntax.Decls {
		switch node := node.(type) {
		case *ast.FuncDecl:
			name := receiverName(node)
			if name != "" && methods[name] == nil {
				methods[name] = node
			}
		case *ast.GenDecl:
			if node.Tok == token.TYPE {
				s.checkType(node, methods)
			}
		}
	}
}

// checkType reports the type in gen whose method comes first, when a method
// above gen is declared on it.
func (s source) checkType(gen *ast.GenDecl, methods map[string]*ast.FuncDecl) {
	var first decl
	for _, spec := range gen.Specs {
		spec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		fn := methods[spec.Name.Name]
		if fn != nil && (first.fn == nil || fn.Pos() < first.fn.Pos()) {
			first = decl{gen, spec, fn}
		}
	}
	if first.fn == nil {
		return
	}
	name := first.spec.Name.Name
	diag := analysis.Diagnostic{
		Pos: first.spec.Pos(),
		End: first.spec.End(),
		Message: "type " + name + " should be declared before method " +
			first.fn.Name.Name,
	}
	// A declaration reads best after the names it mentions, so the fix is
	// withheld when the move would put it above one.
	if out := s.edits(first); out != nil {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Move the declaration above " + first.fn.Name.Name,
			TextEdits: out,
		}}
	}
	s.pass.Report(diag)
}

// receiverName returns the name of the type a method is declared on, and "" if
// there is none.
//
// A receiver is an identifier, a pointer to one, or a type instance of one, so
// its base identifier names the package-level type.
func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	e := fn.Recv.List[0].Type
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	switch x := e.(type) {
	case *ast.IndexExpr:
		e = x.X
	case *ast.IndexListExpr:
		e = x.X
	}
	ident, ok := e.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

// mentionsBetween reports whether m's declaration mentions a name declared
// between its method and itself.
func (s source) mentionsBetween(m decl) bool {
	var crossed bool
	ast.Inspect(m.gen, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj := s.pass.TypesInfo.ObjectOf(ident)
		if obj != nil && m.fn.Pos() < obj.Pos() && obj.Pos() < m.gen.Pos() {
			crossed = true
		}
		return !crossed
	})
	return crossed
}

// edits returns the edits that move m's declaration above m's method, comments
// included, or nil when the move would put the declaration above a name it
// mentions, and when the file on disk is not the one that was parsed.
func (s source) edits(m decl) []analysis.TextEdit {
	if s.mentionsBetween(m) {
		return nil
	}
	// The file may have changed since it was parsed, and then every offset
	// below points into the wrong text.
	if s.Size() != len(s.src) {
		return nil
	}
	var (
		start = s.blockStart(m.gen.Doc, m.gen.Pos())
		end   = lineEnd(s.src, s.Offset(m.gen.End()))
		at    = s.blockStart(m.fn.Doc, m.fn.Pos())
	)
	// The declaration takes the blank line above it, so the gap it leaves is
	// one blank line between its neighbors rather than two.
	del := blankLineBefore(s.src, start)
	moved := bytes.Clone(s.src[start:end])
	moved = append(bytes.TrimSuffix(moved, []byte("\n")), '\n', '\n')
	cut := analysis.TextEdit{Pos: s.Pos(del), End: s.Pos(end)}
	// A declaration with no blank line below it leaves its neighbors joined,
	// so the cut leaves one blank line behind.
	if end < len(s.src) && s.src[end] != '\n' {
		cut.NewText = []byte("\n")
	}
	return []analysis.TextEdit{cut, {
		Pos:     s.Pos(at),
		NewText: moved,
	}}
}

// blockStart returns the offset where a declaration's comment block begins:
// its doc comment, and every comment one blank line above that.
func (s source) blockStart(doc *ast.CommentGroup, pos token.Pos) int {
	if doc != nil {
		pos = doc.Pos()
	}
	start := s.Offset(pos)
	for {
		group := s.commentAbove(start)
		if group == nil {
			return start
		}
		start = lineStart(s.src, s.Offset(group.Pos()))
	}
}

// commentAbove returns the comment group exactly one blank line above start,
// or nil. A comment that trails code on its line is not one, and neither is a
// comment with anything else below it.
func (s source) commentAbove(start int) (above *ast.CommentGroup) {
	for _, group := range s.syntax.Comments {
		pos, end := s.Offset(group.Pos()), s.Offset(group.End())
		if end >= start {
			continue
		}
		if len(bytes.TrimSpace(s.src[lineStart(s.src, pos):pos])) != 0 {
			continue
		}
		if string(s.src[end:start]) != "\n\n" {
			continue
		}
		if above == nil || group.Pos() > above.Pos() {
			above = group
		}
	}
	return above
}

// lineStart returns the offset of the start of the line at off.
func lineStart(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}

// lineEnd returns the offset just past the newline that ends the line at off,
// and the length of a last line that has none.
func lineEnd(src []byte, off int) int {
	for off < len(src) && src[off] != '\n' {
		off++
	}
	if off < len(src) {
		off++
	}
	return off
}

// blankLineBefore returns the offset of the blank line above off, or off when
// the line above holds anything. A method precedes the declaration, so off is
// never the first byte of the file.
func blankLineBefore(src []byte, off int) int {
	from := lineStart(src, off-1)
	if len(bytes.TrimSpace(src[from:off])) == 0 {
		return from
	}
	return off
}
