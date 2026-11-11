package main

import (
	"bytes"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// liftOverlongComments moves a trailing // comment onto the line above the
// code it trails. It does this when the comment pushes the line past the width
// limit.
//
// Trailing comments live in file.Comments. The printer decides whether a
// comment trails or leads by comparing its line with the preceding token's
// line. Specs, fields, and imports also hold the group in their Comment field,
// so a lifted comment has to drop that attachment.
//
// Lifting is two edits. A line boundary goes where the code begins, which
// pushes the code down. A Slash goes on the line the code used to occupy.
func liftOverlongComments(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
		snap    = r.snapshot()
	)
	// The earliest position on each line is where its code begins. A line
	// can begin with a closer, whose position no node reports as its own,
	// so the delimiters are recorded too.
	var (
		earliestOnLine = make(map[int]token.Pos)
		docAt          = make(map[token.Pos]ast.Node)
	)
	note := func(p token.Pos) {
		if !p.IsValid() {
			return
		}
		at := lineAt(tokFile, p)
		if prev, ok := earliestOnLine[at]; !ok || p < prev {
			earliestOnLine[at] = p
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		note(n.Pos())
		switch n := n.(type) {
		case *ast.BlockStmt:
			note(n.Rbrace)
		case *ast.CompositeLit:
			note(n.Rbrace)
		case *ast.CallExpr:
			note(n.Rparen)
		case *ast.IndexExpr:
			note(n.Rbrack)
		case *ast.SliceExpr:
			note(n.Rbrack)
		case *ast.ParenExpr:
			note(n.Rparen)
		case *ast.GenDecl:
			note(n.Rparen)
			docAt[n.Pos()] = n
		case *ast.FuncDecl, *ast.ValueSpec, *ast.TypeSpec, *ast.Field:
			docAt[n.Pos()] = n
		}
		return true
	})
	trailing := trailedBy(file)
	l := &lifter{
		tokFile:  tokFile,
		file:     file,
		snap:     snap,
		earliest: earliestOnLine,
		docAt:    docAt,
	}
	var lifts []lift
	for _, cg := range file.Comments {
		if found, ok := l.lift(cg); ok {
			lifts = append(lifts, found)
		}
	}
	for _, l := range lifts {
		addNewline(tokFile, tokFile.Pos(l.codeOff))
		newSlash := tokFile.Pos(l.codeOff - 1)
		for _, c := range l.cg.List {
			c.Slash = newSlash
		}
	}
	// Fold a lifted comment into the group it now abuts, so the rules that
	// read doc comments see it where the reader does.
	for _, l := range lifts {
		group := l.cg
		// A spec that still claims a comment it no longer trails pads
		// the whole aligned run out to the comment column.
		dropTrailingComment(trailing[group])
		if !opensLine(tokFile, file, group.Pos()) {
			continue
		}
		if prev := groupAbove(tokFile, file, group); prev != nil {
			prev.List = append(prev.List, group.List...)
			file.Comments = slices.DeleteFunc(
				file.Comments,
				func(cg *ast.CommentGroup) bool { return cg == group },
			)
			group = prev
		}
		setDoc(l.decl, group)
	}
}

// lift is one comment group that moves above the code it trails.
type lift struct {
	cg      *ast.CommentGroup
	codeOff int
	decl    ast.Node
}

// lifter is the state one file's lifts are found with.
type lifter struct {
	tokFile  *token.File
	file     *ast.File
	snap     *snapshot
	earliest map[int]token.Pos
	docAt    map[token.Pos]ast.Node
}

// lift returns the lift for one comment group, and whether it can move.
func (l *lifter) lift(cg *ast.CommentGroup) (lift, bool) {
	if len(cg.List) == 0 {
		return lift{}, false
	}
	first := cg.List[0]
	if !strings.HasPrefix(first.Text, "//") {
		return lift{}, false // block comments can wrap; skip
	}
	slash := lineAt(l.tokFile, first.Slash)
	// The comment has to be what makes the line too long. Moving it off a
	// line the code already fills only puts it somewhere else.
	line := l.snap.text(slash)
	code := line[:bytes.Index(line, []byte(first.Text))]
	// The printer pads a comment to the column of the group's widest code,
	// so the padding is not the code's length.
	code = bytes.TrimRight(code, " \t")
	if fits(line) || !fits(code) {
		return lift{}, false
	}
	// The lift leaves the comment alone on the line the code vacated. A
	// line that already carries a comment has no room for it. The printer
	// keeps the two in one group, and a line comment in that group forces
	// a break nobody asked for.
	if commentBefore(l.tokFile, l.file, first) {
		return lift{}, false
	}
	codePos, ok := l.earliest[slash]
	if !ok || codePos >= first.Slash {
		return lift{}, false // nothing before the comment on this line
	}
	// The lift puts the comment above the code on its line. That only reads
	// as "above the code it trails" when the line starts something. On a
	// continuation line, or on a line that opens with a closer, the comment
	// would land inside the expression it trailed.
	if continuesLine(l.snap, slash) {
		return lift{}, false
	}
	codeOff := l.tokFile.Offset(codePos)
	if codeOff <= l.tokFile.Offset(l.tokFile.LineStart(slash)) {
		return lift{}, false // code is at column 1
	}
	return lift{cg: cg, codeOff: codeOff, decl: l.docAt[codePos]}, true
}

// continuesLine reports whether the code on line continues what the line above
// it started. It also reports a line that opens with a closer. A comment
// lifted off such a line would sit between an operator and its operand.
func continuesLine(snap *snapshot, line int) bool {
	code := strings.TrimSpace(string(snap.text(line)))
	if code == "" {
		return true
	}
	switch code[0] {
	case '}', ')', ']':
		return true
	}
	prev := strings.TrimSpace(string(snap.text(line - 1)))
	if prev == "" {
		return false
	}
	return strings.ContainsRune("&|+-*/%,([=:.<>^", rune(prev[len(prev)-1]))
}

// trailedBy indexes the nodes that hold a comment as their trailing one.
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

// groupAbove returns the comment group on the line directly above cg. A parse
// reads the two as one group. A group that trails code does not count. It
// belongs to the code it trails, and folding cg into it would carry cg's
// comment up to that line. A lifted comment's Slash points into the indent of
// the line it moved to, so extent is measured from the slashes.
func groupAbove(tokFile *token.File, file *ast.File, cg *ast.CommentGroup) *ast.CommentGroup {
	want := lineAt(tokFile, cg.List[0].Slash) - 1
	for _, other := range file.Comments {
		if other == cg || len(other.List) == 0 {
			continue
		}
		if lineAt(tokFile, other.List[len(other.List)-1].Slash) != want {
			continue
		}
		if !opensLine(tokFile, file, other.Pos()) {
			continue
		}
		if aloneOnLine(tokFile, file, other.End()) {
			return other
		}
	}
	return nil
}

// commentBefore reports whether another comment shares c's line.
func commentBefore(tokFile *token.File, file *ast.File, c *ast.Comment) bool {
	at := lineAt(tokFile, c.Slash)
	for _, cg := range file.Comments {
		for _, other := range cg.List {
			if other != c && lineAt(tokFile, other.Slash) == at {
				return true
			}
		}
	}
	return false
}

// aloneOnLine reports whether no code shares the line pos ends on.
func aloneOnLine(tokFile *token.File, file *ast.File, pos token.Pos) bool {
	var (
		at = lineAt(tokFile, pos)
		ok = true
	)
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if p := n.Pos(); p.IsValid() && p > pos && lineAt(tokFile, p) == at {
			ok = false
		}
		return ok
	})
	return ok
}

// opensLine reports whether no code begins on pos's line before pos.
func opensLine(tokFile *token.File, file *ast.File, pos token.Pos) (ok bool) {
	at := lineAt(tokFile, pos)
	ok = true
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		p := n.Pos()
		if p.IsValid() && p < pos && lineAt(tokFile, p) == at {
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

// reflowComments reflows the prose runs of doc comments. Anything isProseBody
// rejects is left as it was written.
func reflowComments(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
		snap    = r.snapshot()
	)
	ast.Inspect(file, func(n ast.Node) bool {
		var (
			doc      *ast.CommentGroup
			declLine int
		)
		switch d := n.(type) {
		case *ast.File:
			doc = d.Doc
			declLine = lineAt(tokFile, d.Package)
		case *ast.FuncDecl:
			doc = d.Doc
			declLine = lineAt(tokFile, d.Pos())
		case *ast.TypeSpec:
			doc = d.Doc
			declLine = lineAt(tokFile, d.Pos())
		case *ast.ValueSpec:
			doc = d.Doc
			declLine = lineAt(tokFile, d.Pos())
		case *ast.GenDecl:
			doc = d.Doc
			declLine = lineAt(tokFile, d.Pos())
		default:
			return true
		}
		if doc == nil || len(doc.List) == 0 {
			return true
		}
		lastLine := lineAt(tokFile, doc.List[len(doc.List)-1].End())
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

// isDirective reports whether a comment body is a directive for a compiler or
// a linter rather than prose.
func isDirective(body string) bool {
	prefixes := []string{"go:", "nolint", "export ", "+build", "line "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(body, prefix) {
			return true
		}
	}
	return false
}

// indentAt returns the leading whitespace of the line where c renders.
// go/printer can add a line break the position table does not carry, so the
// line is found by its text rather than by c.Slash.
func indentAt(snap *snapshot, c *ast.Comment) []byte {
	var (
		text = []byte(c.Text)
		line = snap.line(c.Slash)
	)
	for line <= len(snap.lines) && !bytes.Contains(snap.text(line), text) {
		line++
	}
	return extractIndent(snap.text(line))
}

// reflowGroup reflows a single doc comment group in place. A reflowed prose
// run collapses N original lines into M output lines. The token file gains or
// loses that many line boundaries, so the comments that follow keep their line
// adjacency to the run.
func reflowGroup(tokFile *token.File, doc *ast.CommentGroup, snap *snapshot) {
	// A block comment in the group is content the author placed.
	for _, c := range doc.List {
		if !strings.HasPrefix(c.Text, "//") {
			return
		}
	}
	var (
		first  = doc.List[0]
		indent = indentAt(snap, first)
	)
	// An empty comment at this indent has to fit, or there is no room to
	// wrap the prose into.
	if !fits(commentLine(indent, "")) {
		return
	}
	var newList []*ast.Comment
	var i int
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
		origs := doc.List[start:i]
		// An inline annotation in the prose is placed relative to the
		// words around it. An analyzer marker such as an ERROR comment
		// is one. Rewrapping the run would move it.
		if annotated(words) {
			newList = append(newList, origs...)
			continue
		}
		if len(words) == 0 {
			continue
		}
		wrapped := wrapComment(words, indent)
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

// reflowSlashes returns m positions on m consecutive lines for a prose run
// that occupied origs. A run that shrinks has its surplus lines merged away. A
// run that grows has its extra lines carved out of the last original comment's
// own bytes. It returns nil when there is no room for the extra lines, which
// leaves the run as it was.
func reflowSlashes(tokFile *token.File, origs []*ast.Comment, m int) []token.Pos {
	var (
		n   = len(origs)
		out = make([]token.Pos, 0, m)
	)
	for k := 0; k < m && k < n; k++ {
		out = append(out, origs[k].Slash)
	}
	if m == n {
		return out
	}
	if m < n {
		// Collapse the surplus source lines into the last kept line so
		// positions after this run remain adjacent to it.
		lastKept := lineAt(tokFile, origs[m-1].Slash)
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

// allProse reports whether every wrapped line still reads as prose. A line can
// open with a build constraint, a directive, or a list marker. The wrap then
// moved a word to the head of a line, and that changes what the comment is.
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

// isNumberedItem reports whether s opens a numbered list item. That is digits,
// then a period or a right parenthesis, then a space or the end of the line.
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

// annotated reports whether a prose run carries an inline block annotation,
// whose position within the run is content.
func annotated(words []string) bool {
	for _, w := range words {
		if strings.Contains(w, "/*") || strings.Contains(w, "*/") {
			return true
		}
	}
	return false
}

// wrapComment packs words into comment lines at the indent the comment
// renders at. It asks the width rules after every word, so the definition of a
// long line stays linelen's. A word longer than the limit gets a line of its
// own. It is never broken.
func wrapComment(words []string, indent []byte) (lines []string) {
	var line string
	for _, w := range words {
		if line == "" {
			line = w
			continue
		}
		next := line + " " + w
		if !fits(commentLine(indent, next)) {
			lines = append(lines, line)
			line = w
			continue
		}
		line = next
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// commentLine returns the bytes a comment holding text renders as at the given
// indent.
func commentLine(indent []byte, text string) []byte {
	return []byte(string(indent) + "// " + text)
}
