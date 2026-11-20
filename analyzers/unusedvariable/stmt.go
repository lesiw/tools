package unusedvariable

import (
	"go/ast"
	"go/token"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// around is what a statement's lines hold besides the statement: the parent
// syntax that shares them, the statements beside it, and the comments on them.
type around struct {
	first, last int
	left, right token.Pos // parent syntax on the first and last line
	prev, next  token.Pos // statements beside it on those lines
	before      []*ast.Comment
	after       []*ast.Comment
	listed      bool
}

// deleteStmt returns the edits that remove a statement and the comments on its
// line, or nil when the line also holds syntax that has to stay. A line is not
// all or nothing: a statement that shares one with another keeps that other
// one and loses only its own text.
func (s *syntax) deleteStmt(stmt ast.Stmt) []analysis.TextEdit {
	a, ok := s.aroundOf(stmt)
	if !ok {
		return nil
	}
	return []analysis.TextEdit{{
		Pos: s.leftLimit(stmt, a),
		End: s.rightLimit(stmt, a),
	}}
}

// aroundOf returns what the statement's lines hold, and false when the
// statement sits where nothing can be removed.
func (s *syntax) aroundOf(stmt ast.Stmt) (around, bool) {
	a := around{first: s.line(stmt.Pos()), last: s.line(stmt.End())}
	if ok := s.delimiters(stmt, &a); !ok {
		return around{}, false
	}
	if a.listed {
		a.prev, a.next = s.neighbors(stmt, a.first, a.last)
	}
	a.before, a.after = s.commentsOn(stmt, a.first, a.last)
	return a, true
}

// delimiters records the parent syntax that sits on the statement's lines, and
// whether the parent holds a statement list.
func (s *syntax) delimiters(stmt ast.Stmt, a *around) bool {
	switch parent := s.parent(stmt).(type) {
	case *ast.BlockStmt:
		s.inside(a, parent.Lbrace, parent.Rbrace, true)
	case *ast.CaseClause:
		body := s.parent(parent).(*ast.BlockStmt)
		s.inside(a, parent.Colon, body.Rbrace, true)
	case *ast.CommClause:
		body := s.parent(parent).(*ast.BlockStmt)
		s.inside(a, parent.Colon, body.Rbrace, true)
	case *ast.ForStmt:
		s.forDelimiters(parent, stmt, a)
	case *ast.IfStmt:
		s.ifDelimiters(parent, stmt, a)
	case *ast.SwitchStmt:
		s.inside(a, parent.Switch, parent.Body.Lbrace, false)
	case *ast.TypeSwitchStmt:
		s.inside(a, parent.Switch, parent.Body.Lbrace, false)
	default:
		return false
	}
	return true
}

// inside records the two delimiters around a statement list, or around the
// statement when the parent holds no list.
func (s *syntax) inside(a *around, open, closer token.Pos, listed bool) {
	a.left, a.right = s.onLines(a.first, a.last, open, closer)
	a.listed = listed
}

// onLines returns the delimiters that sit on the given lines.
func (s *syntax) onLines(first, last int, open, closer token.Pos) (left, right token.Pos) {
	if s.line(open) == first {
		left = open
	}
	if s.line(closer) == last {
		right = closer
	}
	return left, right
}

// forDelimiters records what a for statement's header puts on the lines of its
// init or post statement. The three parts are not a statement list, so a
// neighbor shares a delimiter with the statement and a comment between them
// cannot be removed.
func (s *syntax) forDelimiters(f *ast.ForStmt, stmt ast.Stmt, a *around) {
	s.inside(a, f.For, f.Body.Lbrace, false)
	if stmt == f.Init && f.Cond != nil && s.line(f.Cond.Pos()) == a.last {
		a.next = f.Cond.Pos()
	}
	if stmt == f.Post && f.Cond != nil && s.line(f.Cond.End()) == a.first {
		a.prev = f.Cond.End()
	}
}

// ifDelimiters records what an if statement puts on the lines of its init
// statement, which is the only part of the header this rule removes.
func (s *syntax) ifDelimiters(cond *ast.IfStmt, stmt ast.Stmt, a *around) {
	s.inside(a, cond.If, cond.Body.Lbrace, false)
	// The condition shares a line with the init rather than a delimiter, so a
	// comment between them cannot be removed.
	if stmt == cond.Init && s.line(cond.Cond.Pos()) == a.last {
		a.next = cond.Cond.Pos()
	}
}

// neighbors returns the statements beside stmt that share its lines.
func (s *syntax) neighbors(stmt ast.Stmt, first, last int) (prev, next token.Pos) {
	var (
		list  = statements(s.parent(stmt))
		index = slices.Index(list, stmt)
	)
	if index < 0 {
		return token.NoPos, token.NoPos
	}
	if index > 0 && s.line(list[index-1].End()) == first {
		prev = list[index-1].End()
	}
	if index+1 < len(list) && s.line(list[index+1].Pos()) == last {
		next = list[index+1].Pos()
	}
	return prev, next
}

// commentsOn returns the comments before and after the statement that sit on
// its first and last lines.
func (s *syntax) commentsOn(stmt ast.Stmt, first, last int) (before, after []*ast.Comment) {
	for _, group := range s.file(stmt).Comments {
		for _, comment := range group.List {
			if s.line(comment.End()) < first {
				continue
			} else if s.line(comment.Pos()) > last {
				return before, after // no later comment can come back
			}
			switch {
			case s.line(comment.End()) == first && comment.End() <= stmt.Pos():
				before = append(before, comment)
			case s.line(comment.Pos()) == last && comment.Pos() >= stmt.End():
				after = append(after, comment)
			}
		}
	}
	return before, after
}

// leftLimit returns where the edit that removes the statement begins.
func (s *syntax) leftLimit(stmt ast.Stmt, a around) token.Pos {
	switch {
	case a.prev.IsValid():
		// A statement precedes this one on the line, so a comment between
		// them cannot be told from one that follows it.
		return stmt.Pos()
	case a.left.IsValid():
		if window := within(a.before, a.left, stmt.Pos()); len(window) > 0 {
			return window[0].Pos()
		}
		return stmt.Pos()
	}
	// Nothing else is on the line, so the line goes with it.
	pos := stmt.Pos()
	for s.line(pos) == a.first {
		pos--
	}
	return pos + 1
}

// rightLimit returns where the edit that removes the statement ends.
func (s *syntax) rightLimit(stmt ast.Stmt, a around) token.Pos {
	switch {
	case a.next.IsValid():
		return stmt.End()
	case a.right.IsValid():
		if window := within(a.after, stmt.End(), a.right); len(window) > 0 {
			return window[len(window)-1].End()
		}
		return stmt.End()
	}
	// Nothing else is on the line, so the line goes with it, unless syntax
	// before the statement keeps its newline.
	pos := stmt.End()
	for s.line(pos) == a.last {
		pos++
	}
	if a.left.IsValid() || a.prev.IsValid() {
		pos--
	}
	return pos
}

// eol returns the position just past the newline that ends the line of pos.
func (s *syntax) eol(pos token.Pos) token.Pos {
	line := s.line(pos)
	for s.line(pos) == line {
		pos++
	}
	return pos
}

// statements returns the statement list a node holds, or nil for a node that
// holds none.
func statements(n ast.Node) []ast.Stmt {
	switch n := n.(type) {
	case *ast.BlockStmt:
		return n.List
	case *ast.CaseClause:
		return n.Body
	case *ast.CommClause:
		return n.Body
	}
	return nil
}

// within returns the comments that fall between start and end.
func within(comments []*ast.Comment, start, end token.Pos) (out []*ast.Comment) {
	for _, comment := range comments {
		if comment.Pos() > start && comment.End() <= end {
			out = append(out, comment)
		}
	}
	return out
}
