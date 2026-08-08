package main

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"unicode/utf8"
)

// liftOverlongComments moves trailing inline // comments that push a line
// past the width limit to their own line above the code they trail. Trailing
// comments are stored in file.Comments (not on the statement AST nodes) and
// the printer decides "trailing vs. above" by comparing the comment's line
// with the preceding token's line. To lift, we insert a line boundary at
// the offset where the code begins on the overlong line — that shifts the
// code down to a new line — then set the comment's Slash to a position on
// the original line (now containing only the indent). The printer then emits
// the comment on its own line, with the code following on the next line.
func liftOverlongComments(tokFile *token.File, file *ast.File, snap *snapshot) {
	// Collect, per line, the earliest node position on that line. That
	// offset is where code begins on the line — the split point for lifting.
	var (
		earliestOnLine = make(map[int]token.Pos)
		docAt          = make(map[token.Pos]ast.Node)
	)
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		p := n.Pos()
		if !p.IsValid() {
			return true
		}
		line := tokFile.Line(p)
		if prev, ok := earliestOnLine[line]; !ok || p < prev {
			earliestOnLine[line] = p
		}
		switch n.(type) {
		case *ast.GenDecl, *ast.FuncDecl, *ast.ValueSpec, *ast.TypeSpec,
			*ast.Field:
			if _, ok := docAt[p]; !ok {
				docAt[p] = n
			}
		}
		return true
	})
	trailing := trailedBy(file)

	type lift struct {
		cg      *ast.CommentGroup
		codeOff int
		decl    ast.Node
	}
	var lifts []lift

	for _, cg := range file.Comments {
		if len(cg.List) == 0 {
			continue
		}
		first := cg.List[0]
		if !strings.HasPrefix(first.Text, "//") {
			continue // block comments can wrap; skip
		}
		slash := tokFile.Line(first.Slash)
		if snap.width(slash) <= cfg.Len {
			continue
		}
		codePos, ok := earliestOnLine[slash]
		if !ok || codePos >= first.Slash {
			continue // nothing before the comment on this line
		}
		var (
			codeOff      = tokFile.Offset(codePos)
			lineStartOff = tokFile.Offset(tokFile.LineStart(slash))
		)
		if codeOff <= lineStartOff {
			continue // code is at column 1; no indent to carve the lift into
		}
		lifts = append(lifts, lift{
			cg: cg, codeOff: codeOff, decl: docAt[codePos],
		})
	}

	// Apply the lifts. addNewline inserts a new boundary at the code's
	// offset: code shifts to the next line, and offsets before it stay on
	// the original line. Setting the comment's Slash to codeOff-1 places it
	// on that original line — now a "virtual" line above the code.
	for _, l := range lifts {
		addNewline(tokFile, tokFile.Pos(l.codeOff))
		newSlash := tokFile.Pos(l.codeOff - 1)
		for _, c := range l.cg.List {
			c.Slash = newSlash
		}
	}

	// A lifted comment now occupies a whole line above the code it used
	// to trail. Fold it into the group it has come to abut and hang it
	// off the declaration below it, so the rules that read doc comments
	// see the comment where the reader does.
	for _, l := range lifts {
		group := l.cg
		// go/printer pads a spec that trails a comment out to the
		// comment column, so a spec still claiming a comment it no
		// longer trails widens the whole aligned run.
		dropTrailingComment(trailing[group])
		if !opensLine(tokFile, file, group.Pos()) {
			continue
		}
		if prev := groupAbove(tokFile, file, group); prev != nil {
			prev.List = append(prev.List, group.List...)
			file.Comments = slices.DeleteFunc(file.Comments,
				func(cg *ast.CommentGroup) bool { return cg == group },
			)
			group = prev
		}
		setDoc(l.decl, group)
	}
}

// trailedBy indexes the nodes that hold a comment as their trailing
// one, by that comment.
func trailedBy(file *ast.File) map[*ast.CommentGroup]ast.Node {
	out := make(map[*ast.CommentGroup]ast.Node)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			if n.Comment != nil {
				out[n.Comment] = n
			}
		case *ast.ValueSpec:
			if n.Comment != nil {
				out[n.Comment] = n
			}
		case *ast.TypeSpec:
			if n.Comment != nil {
				out[n.Comment] = n
			}
		case *ast.ImportSpec:
			if n.Comment != nil {
				out[n.Comment] = n
			}
		}
		return true
	})
	return out
}

// dropTrailingComment releases the comment a node claims to trail.
func dropTrailingComment(n ast.Node) {
	switch n := n.(type) {
	case *ast.Field:
		n.Comment = nil
	case *ast.ValueSpec:
		n.Comment = nil
	case *ast.TypeSpec:
		n.Comment = nil
	case *ast.ImportSpec:
		n.Comment = nil
	}
}

// groupAbove returns the comment group that owns the line directly
// above cg, which a parse would read as one group with it. A group
// trailing code is not one: it belongs to the code it trails, and
// folding cg into it would carry cg's comment up to that line. A lifted
// comment's Slash points into the indent of the line it moved to, so
// extent is measured from the slashes rather than from End.
func groupAbove(tokFile *token.File, file *ast.File, cg *ast.CommentGroup) *ast.CommentGroup {
	want := tokFile.Line(cg.List[0].Slash) - 1
	for _, other := range file.Comments {
		if other == cg || len(other.List) == 0 {
			continue
		}
		if tokFile.Line(other.List[len(other.List)-1].Slash) != want {
			continue
		}
		if opensLine(tokFile, file, other.Pos()) {
			return other
		}
	}
	return nil
}

// opensLine reports whether no code begins on pos's line before pos.
func opensLine(tokFile *token.File, file *ast.File, pos token.Pos) (ok bool) {
	line := tokFile.Line(pos)
	ok = true
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		p := n.Pos()
		if p.IsValid() && p < pos && tokFile.Line(p) == line {
			ok = false
		}
		return ok
	})
	return ok
}

// setDoc hangs cg on n as its doc comment.
func setDoc(n ast.Node, cg *ast.CommentGroup) {
	switch n := n.(type) {
	case *ast.GenDecl:
		n.Doc = cg
	case *ast.FuncDecl:
		n.Doc = cg
	case *ast.ValueSpec:
		n.Doc = cg
	case *ast.TypeSpec:
		n.Doc = cg
	case *ast.Field:
		n.Doc = cg
	}
}

// reflowComments reflows doc comment paragraphs. It preserves code
// blocks, headings, list items, and directives.
func reflowComments(fset *token.FileSet, tokFile *token.File, file *ast.File, snap *snapshot) {
	ast.Inspect(file, func(n ast.Node) bool {
		var (
			doc      *ast.CommentGroup
			declLine int
		)
		switch d := n.(type) {
		case *ast.File:
			doc = d.Doc
			declLine = fset.Position(d.Package).Line
		case *ast.FuncDecl:
			doc = d.Doc
			declLine = fset.Position(d.Pos()).Line
		case *ast.TypeSpec:
			doc = d.Doc
			declLine = fset.Position(d.Pos()).Line
		case *ast.ValueSpec:
			doc = d.Doc
			declLine = fset.Position(d.Pos()).Line
		case *ast.GenDecl:
			doc = d.Doc
			declLine = fset.Position(d.Pos()).Line
		default:
			return true
		}
		if doc == nil || len(doc.List) == 0 {
			return true
		}
		lastLine := fset.Position(doc.List[len(doc.List)-1].End()).Line
		if lastLine < declLine-1 {
			return true // not a doc comment
		}
		for _, c := range doc.List {
			if isDirective(stripCommentPrefix(c.Text)) {
				return true
			}
		}
		reflowGroup(tokFile, doc, snap)
		return true
	})
}

// isDirective reports whether a comment body is a compiler or linter
// directive rather than prose.
func isDirective(body string) bool {
	for _, prefix := range []string{
		"go:", "nolint", "export ", "+build", "line ",
	} {
		if strings.HasPrefix(body, prefix) {
			return true
		}
	}
	return false
}

// reflowGroup reflows a single doc comment group in place. Each reflowed
// prose run collapses N original lines into M output lines; the token
// file gains or loses that many line boundaries so the comments that
// follow keep their line adjacency to the run.
func reflowGroup(tokFile *token.File, doc *ast.CommentGroup, snap *snapshot) {
	first := doc.List[0]
	if !strings.HasPrefix(first.Text, "//") {
		return // block comment; don't reflow
	}
	textWidth := cfg.Len - width(snap.indent(first.Slash)) - len("// ")
	if textWidth < 1 {
		return
	}
	var (
		newList []*ast.Comment
		i       int
	)
	for i < len(doc.List) {
		if !isProseBody(stripCommentPrefix(doc.List[i].Text)) {
			newList = append(newList, doc.List[i])
			i++
			continue
		}
		var (
			words []string
			start = i
		)
		for i < len(doc.List) {
			body := stripCommentPrefix(doc.List[i].Text)
			if !isProseBody(body) {
				break
			}
			words = append(words, strings.Fields(body)...)
			i++
		}
		var (
			origs   = doc.List[start:i]
			wrapped = wrapWordsToLines(words, textWidth)
		)
		if !allProse(wrapped) {
			newList = append(newList, origs...)
			continue
		}
		slashes := reflowSlashes(tokFile, origs, len(wrapped))
		if slashes == nil {
			newList = append(newList, origs...)
			continue
		}
		for k, line := range wrapped {
			newList = append(newList, &ast.Comment{
				Slash: slashes[k],
				Text:  "// " + line,
			})
		}
	}
	doc.List = newList
}

// reflowSlashes returns m positions on m consecutive lines for a prose
// run that occupied origs. When the run shrinks, the surplus lines are
// merged away; when it grows, the extra lines are carved out of the last
// original comment's own bytes. It returns nil when there is no room for
// the extra lines, leaving the run to be kept as it was.
func reflowSlashes(tokFile *token.File, origs []*ast.Comment, m int) []token.Pos {
	n := len(origs)
	out := make([]token.Pos, 0, m)
	for k := 0; k < m && k < n; k++ {
		out = append(out, origs[k].Slash)
	}
	if m == n {
		return out
	}
	if m < n {
		// Collapse the surplus source lines into the last kept line so
		// positions after this run remain adjacent to it.
		lastKept := tokFile.Line(origs[m-1].Slash)
		for range n - m {
			tokFile.MergeLine(lastKept)
		}
		return out
	}
	last := origs[n-1]
	if m-n >= len(last.Text) {
		return nil
	}
	off := tokFile.Offset(last.Slash)
	for k := 1; k <= m-n; k++ {
		addNewline(tokFile, tokFile.Pos(off+k))
		out = append(out, tokFile.Pos(off+k))
	}
	return out
}

// allProse reports whether every wrapped line still reads as prose. A
// line that opens with a build constraint, a directive, or a list
// marker means the wrap moved a word to the head of a line where it
// changes what the comment is.
func allProse(lines []string) bool {
	for _, line := range lines {
		if isDirective(line) || !isProseBody(line) {
			return false
		}
	}
	return true
}

// stripCommentPrefix returns the body of a // comment, with the leading "//"
// and optional single space removed.
func stripCommentPrefix(text string) string {
	body := strings.TrimPrefix(text, "//")
	if strings.HasPrefix(body, " ") {
		return body[1:]
	}
	return body
}

// isProseBody reports whether a comment body is regular prose eligible for
// reflowing (not blank, code, heading, or list item).
func isProseBody(body string) bool {
	if body == "" {
		return false
	}
	if strings.HasPrefix(body, "\t") || strings.HasPrefix(body, "    ") {
		return false // code block
	}
	if strings.HasPrefix(body, "# ") {
		return false // heading
	}
	if strings.HasPrefix(body, "   ") {
		return false // list item continuation
	}
	item := strings.TrimLeft(body, " ")
	if strings.HasPrefix(item, "- ") || strings.HasPrefix(item, "* ") {
		return false // bullet list
	}
	return !isNumberedItem(item)
}

// isNumberedItem reports whether s opens a numbered list item: digits
// followed by a period or right paren, then a space or end of line.
func isNumberedItem(s string) bool {
	var i int
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) || (s[i] != '.' && s[i] != ')') {
		return false
	}
	return i+1 == len(s) || s[i+1] == ' '
}

// wrapWordsToLines wraps words into lines of at most width characters.
func wrapWordsToLines(words []string, width int) (lines []string) {
	if len(words) == 0 {
		return nil
	}
	var (
		line      = words[0]
		lineWidth = utf8.RuneCountInString(words[0])
	)
	for _, w := range words[1:] {
		wWidth := utf8.RuneCountInString(w)
		if lineWidth+1+wWidth > width {
			lines = append(lines, line)
			line = w
			lineWidth = wWidth
			continue
		}
		line += " " + w
		lineWidth += 1 + wWidth
	}
	return append(lines, line)
}
