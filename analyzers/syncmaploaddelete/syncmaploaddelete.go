// Package syncmaploaddelete provides an analysis.Analyzer that reports a
// sync.Map load followed by a delete of the same key.
//
// LoadAndDelete returns what Load reports through ok and deletes the key in
// the same call, and it has been in sync.Map since Go 1.15.
//
// A pair of calls leaves a window between the load and the delete, and a value
// another goroutine stores there is deleted unseen. The fix closes the window
// by making the pair one call. It is withheld where the edit would lose a
// comment, leave a declared name unread, or move the delete to a statement
// that a goto can reach without the load; the README lists the shapes that
// report without one.
//
// The rule is adapted from go-critic's syncMapLoadAndDelete, which reports the
// same pair when it is written above the if, and offers no fix.
package syncmaploaddelete

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "syncmaploaddelete",
	Doc:  "reports a sync.Map load followed by a delete of the same key",
	Run:  run,
}

// load is a call to a sync.Map's Load, with the statement that assigns its
// results.
type load struct {
	name    *ast.Ident      // the selector that names Load
	m       ast.Expr        // the receiver
	key     ast.Expr        // the key
	ok      ast.Expr        // the operand the presence flag is assigned to
	assign  *ast.AssignStmt // the assignment, or the if's init
	pos     token.Pos       // where the pair starts
	head    bool            // the assignment is the if's init
	labeled bool            // the if carries a label
	chained bool            // the if follows an else
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// LoadAndDelete is in sync.Map from Go 1.15. An empty version means
		// the type checker enforced none.
		v := pass.TypesInfo.FileVersions[file]
		if v != "" && version.Compare(v, "go1.15") < 0 {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				statements(pass, file, n.List)
			case *ast.CaseClause:
				statements(pass, file, n.Body)
			case *ast.CommClause:
				statements(pass, file, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// statements examines one statement list for a load whose key is then deleted.
func statements(pass *analysis.Pass, file *ast.File, list []ast.Stmt) {
	for i, stmt := range list {
		ifStmt, ok := unlabeled(stmt).(*ast.IfStmt)
		if !ok {
			continue
		}
		elses(pass, file, stmt, ifStmt)
		// The delete moves to where the load is written, so a load above the
		// if has to sit directly above it, and that if may not have an init.
		if i == 0 || ifStmt.Init != nil {
			continue
		}
		l, ok := loadOf(pass, unlabeled(list[i-1]))
		if !ok {
			continue
		}
		l.labeled = labeled(stmt)
		l.pos = list[i-1].Pos()
		examine(pass, file, ifStmt, l)
	}
}

// elses examines the if and the ifs its else leads to, where a load can only
// be in the header: an else-if has no statement written above it.
func elses(pass *analysis.Pass, file *ast.File, outer ast.Stmt, ifStmt *ast.IfStmt) {
	for first := true; ifStmt != nil; first = false {
		if l, ok := loadOf(pass, ifStmt.Init); ok {
			l.head = true
			l.labeled = first && labeled(outer)
			l.chained = !first
			l.pos = ifStmt.Pos()
			if first {
				// A label comes before the if it names.
				l.pos = outer.Pos()
			}
			examine(pass, file, ifStmt, l)
		}
		ifStmt, _ = unlabeled(ifStmt.Else).(*ast.IfStmt)
	}
}

// unlabeled returns the statement a label precedes.
func unlabeled(stmt ast.Stmt) ast.Stmt {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = labeled.Stmt
	}
}

// labeled reports whether stmt carries a label.
func labeled(stmt ast.Stmt) bool {
	_, ok := stmt.(*ast.LabeledStmt)
	return ok
}

// examine reports ifStmt when the load's flag guards a delete of the key the
// load read.
func examine(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, l load) {
	if !astexpr.Same(pass, ifStmt.Cond, l.ok) {
		return
	}
	del, ok := deletion(pass, ifStmt.Body, l)
	if !ok {
		return
	}
	d := analysis.Diagnostic{
		Pos:     l.pos,
		End:     ifStmt.End(),
		Message: "load and delete can be replaced by sync.Map.LoadAndDelete",
	}
	if edits, ok := fix(pass, file, ifStmt, l, del); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Replace with LoadAndDelete",
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// deletion returns the delete that removes the key l read, which has to open
// the if's body: the rewrite deletes where the load is, and a delete further
// down runs after the statements above it.
func deletion(pass *analysis.Pass, body *ast.BlockStmt, l load) (*ast.ExprStmt, bool) {
	if len(body.List) == 0 {
		return nil, false
	}
	stmt, ok := unlabeled(body.List[0]).(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	m, _, ok := syncMapCall(pass, call, "Delete")
	key := call.Args[0]
	if !ok || !astexpr.Same(pass, m, l.m) || !astexpr.Same(pass, key, l.key) {
		return nil, false
	}
	return stmt, true
}

// loadOf returns the sync.Map load that stmt assigns, and reports false for
// any other statement.
func loadOf(pass *analysis.Pass, stmt ast.Stmt) (load, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || (assign.Tok != token.DEFINE && assign.Tok != token.ASSIGN) {
		return load{}, false
	}
	if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return load{}, false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return load{}, false
	}
	m, sel, ok := syncMapCall(pass, call, "Load")
	if !ok {
		return load{}, false
	}
	return load{
		name:   sel,
		m:      m,
		key:    call.Args[0],
		ok:     assign.Lhs[1],
		assign: assign,
	}, true
}

// syncMapCall returns the receiver of a call to a method of sync.Map and the
// identifier that names it, and reports false for any other call.
func syncMapCall(pass *analysis.Pass, call *ast.CallExpr, name string) (ast.Expr, *ast.Ident, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Name() != name || !fromSync(fn) {
		return nil, nil, false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || !syncMapPointer(sig.Recv().Type()) {
		return nil, nil, false
	}
	return sel.X, sel.Sel, true
}

// syncMapPointer reports whether t is *sync.Map.
func syncMapPointer(t types.Type) bool {
	ptr, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "Map" && fromSync(obj)
}

// fromSync reports whether obj is declared by the sync package.
func fromSync(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Pkg().Path() == "sync"
}

// fix returns the edits that rename the load and drop the delete, and reports
// false when the edit would lose a comment or run past the width.
func fix(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, l load, del *ast.ExprStmt) ([]analysis.TextEdit, bool) {
	src, ok := sourceOf(pass, ifStmt.Pos())
	if !ok {
		return nil, false
	}
	// A load above a labeled if is deleted where it is written, and a goto can
	// land on the label without running the load.
	if l.labeled && !l.head {
		return nil, false
	}
	// A body that holds nothing else leaves the if no work once the delete is
	// gone, so the load's results decide how it goes.
	if len(ifStmt.Body.List) == 1 {
		return only(pass, file, ifStmt, l, src)
	}
	// A label would be left naming nothing.
	if labeled(ifStmt.Body.List[0]) {
		return nil, false
	}
	// A comment between the brace and the delete would be left describing the
	// statement that follows it.
	if commented(file, ifStmt.Body.Lbrace, del.Pos()) {
		return nil, false
	}
	rename, ok := rename(src, l)
	if !ok {
		return nil, false
	}
	erase, ok := erasure(file, src, del)
	if !ok {
		return nil, false
	}
	return []analysis.TextEdit{rename, erase}, true
}

// only returns the edits for an if whose body holds nothing but the delete.
// The if has no work left, so it goes, and the names the load bound have to
// stay read or go with them.
func only(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, l load, src source) ([]analysis.TextEdit, bool) {
	// An else-if that loses its if takes the else with it.
	if ifStmt.Else != nil || l.chained {
		return nil, false
	}
	value, flag := operands(pass, l.assign)
	if l.head {
		return header(pass, file, ifStmt, l, src, flag)
	}
	used, read := usage(pass, file, ifStmt.End(), flag.obj)
	switch {
	case read || flag.obj == nil:
		// Something after the if reads the flag, or the flag names no
		// variable, so the assignment stays and the if goes.
		return pairEdits(file, ifStmt, l, src, nil)
	case used || !flag.fresh:
		// The if was the last to read the flag, or the assignment does not
		// declare it, so the edit would leave it declared and unread.
		return nil, false
	case value.blank():
		// Nothing reads what the load bound, so one call stands for both
		// statements.
		call, ok := callText(l.m, l.key)
		if !ok {
			return nil, false
		}
		return replacement(file, src, l.assign.Pos(), ifStmt.End(), call)
	}
	// The value is still read, and the flag is not, so the flag is written as
	// a blank.
	edits := []analysis.TextEdit{{
		Pos:     flag.expr.Pos(),
		End:     flag.expr.End(),
		NewText: []byte("_"),
	}}
	if l.assign.Tok == token.DEFINE && !value.fresh {
		// The flag was the only name the assignment declared, so := has
		// nothing left to declare.
		edits = append(edits, analysis.TextEdit{
			Pos:     l.assign.TokPos,
			End:     l.assign.TokPos + 2,
			NewText: []byte("="),
		})
	}
	return pairEdits(file, ifStmt, l, src, edits)
}

// header returns the edits for a load written in the if's header, where the
// condition goes with the if: a declaration goes with it too, and an
// assignment stays.
func header(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, l load, src source, flag operand) ([]analysis.TextEdit, bool) {
	call, oneLine := callText(l.m, l.key)
	if l.assign.Tok == token.DEFINE {
		if !oneLine {
			return nil, false
		}
		return replacement(file, src, ifStmt.Pos(), ifStmt.End(), call)
	}
	// The condition was the flag's last reader, so something after the if has
	// to read it in turn.
	_, read := usage(pass, file, ifStmt.End(), flag.obj)
	if !read && flag.obj != nil {
		return nil, false
	}
	if !oneLine {
		return nil, false
	}
	text, ok := assignment(l.assign, call)
	if !ok {
		return nil, false
	}
	return replacement(file, src, ifStmt.Pos(), ifStmt.End(), text)
}

// assignment returns the source text that assigns a call to the operands the
// load writes, as in v, ok = m.LoadAndDelete(key).
func assignment(assign *ast.AssignStmt, call string) (string, bool) {
	operands := make([]string, len(assign.Lhs))
	for i, operand := range assign.Lhs {
		text, ok := astexpr.OneLine(operand)
		if !ok {
			return "", false
		}
		operands[i] = text
	}
	return strings.Join(operands, ", ") + " = " + call, true
}

// pairEdits returns the edits that rename the load and drop the if, with the
// caller's edits written in the same pass.
func pairEdits(file *ast.File, ifStmt *ast.IfStmt, l load, src source, extra []analysis.TextEdit) ([]analysis.TextEdit, bool) {
	// A comment between the load and the if would be left describing what
	// follows it. One on the load's own line stays where it is.
	if !l.head && commented(file, src.nextLine(l.assign.End()), ifStmt.Pos()) {
		return nil, false
	}
	rename, ok := rename(src, l)
	if !ok {
		return nil, false
	}
	erase, ok := erasure(file, src, ifStmt)
	if !ok {
		return nil, false
	}
	return append(append([]analysis.TextEdit{rename}, extra...), erase), true
}

// rename returns the edit that names LoadAndDelete where Load is written.
func rename(src source, l load) (analysis.TextEdit, bool) {
	if !fits(src, l.name.Pos(), l.name.End(), "LoadAndDelete") {
		return analysis.TextEdit{}, false
	}
	return analysis.TextEdit{
		Pos:     l.name.Pos(),
		End:     l.name.End(),
		NewText: []byte("LoadAndDelete"),
	}, true
}

// replacement returns the edit that puts text in place of a span, and reports
// false when a comment sits in the span or the line would run past the width.
func replacement(file *ast.File, src source, lo, hi token.Pos, text string) ([]analysis.TextEdit, bool) {
	if commented(file, lo, hi) || !fits(src, lo, hi, text) {
		return nil, false
	}
	return []analysis.TextEdit{{Pos: lo, End: hi, NewText: []byte(text)}}, true
}

// operand is one of the two things a load's assignment writes.
type operand struct {
	expr  ast.Expr
	obj   types.Object // nil for a blank identifier and for an unnamed operand
	fresh bool         // the name is declared by this assignment
}

// blank reports whether the operand is the blank identifier.
func (o operand) blank() bool {
	ident, ok := ast.Unparen(o.expr).(*ast.Ident)
	return ok && ident.Name == "_"
}

// operands returns what the two operands of a load's assignment hold.
func operands(pass *analysis.Pass, assign *ast.AssignStmt) (value, flag operand) {
	return operandOf(pass, assign, 0), operandOf(pass, assign, 1)
}

// operandOf returns what one operand of a load's assignment holds.
func operandOf(pass *analysis.Pass, assign *ast.AssignStmt, i int) operand {
	o := operand{expr: assign.Lhs[i]}
	if ident, ok := writing(o.expr); ok {
		o.obj = pass.TypesInfo.ObjectOf(ident)
		o.fresh = o.obj != nil &&
			o.obj.Pos() >= assign.Pos() && o.obj.Pos() < assign.End()
	}
	return o
}

// writing returns the identifier an operand assigns to, and reports false for
// the blank identifier and for anything a fix cannot name.
func writing(e ast.Expr) (*ast.Ident, bool) {
	ident, ok := ast.Unparen(e).(*ast.Ident)
	return ident, ok && ident.Name != "_"
}

// usage reports whether obj is referenced after pos, and whether it is read
// there. A name on the left of an assignment, or in an increment, is written
// and not read.
func usage(pass *analysis.Pass, file *ast.File, pos token.Pos, obj types.Object) (used, read bool) {
	if obj == nil {
		return false, false
	}
	written := writtenNames(file)
	ast.Inspect(file, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok || ident.Pos() <= pos || pass.TypesInfo.ObjectOf(ident) != obj {
			return true
		}
		used = true
		read = read || !written[ident]
		return !read
	})
	return used, read
}

// writtenNames returns the identifiers an assignment or an increment writes to
// without reading them.
func writtenNames(file *ast.File) map[*ast.Ident]bool {
	written := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					written[ident] = true
				}
			}
		case *ast.IncDecStmt:
			if ident, ok := ast.Unparen(n.X).(*ast.Ident); ok {
				written[ident] = true
			}
		}
		return true
	})
	return written
}

// callText returns the source text of m.LoadAndDelete(key) on one line, and
// reports false when the receiver or the key has no one-line form.
func callText(m, key ast.Expr) (string, bool) {
	recv, ok := astexpr.OneLine(m)
	if !ok {
		return "", false
	}
	arg, ok := astexpr.OneLine(key)
	if !ok {
		return "", false
	}
	return recv + ".LoadAndDelete(" + arg + ")", true
}

// source is the text of the file a fix rewrites.
type source struct {
	*token.File
	text []byte
}

// sourceOf returns the file that pos is written in, with its text.
func sourceOf(pass *analysis.Pass, pos token.Pos) (source, bool) {
	file := pass.Fset.File(pos)
	if file == nil {
		return source{}, false
	}
	text, err := pass.ReadFile(file.Name())
	if err != nil {
		return source{}, false
	}
	return source{File: file, text: text}, true
}

// within reports whether every offset is inside the text, and in the order
// given.
func (s source) within(offs ...int) bool {
	for i, off := range offs {
		if off < 0 || off > len(s.text) {
			return false
		}
		if i > 0 && off < offs[i-1] {
			return false
		}
	}
	return true
}

// lineStart returns the offset the line holding pos starts at.
func (s source) lineStart(pos token.Pos) int {
	return s.Offset(s.LineStart(s.Line(pos)))
}

// lineEnd returns the offset of the newline that ends the line holding pos, or
// the length of the text on the last line.
func (s source) lineEnd(pos token.Pos) int {
	line := s.Line(pos)
	if line == s.LineCount() {
		return len(s.text)
	}
	return s.Offset(s.LineStart(line+1)) - 1
}

// nextLine returns the position the line after pos starts at, or the end of
// the text.
func (s source) nextLine(pos token.Pos) token.Pos {
	end := s.lineEnd(pos)
	if end < len(s.text) {
		end++
	}
	return s.Pos(end)
}

// erasure returns the edit that removes stmt, taking its line with it when
// nothing else is written there, and reports false when a comment would go
// with it or the statement shares a line in a shape the edit cannot separate.
func erasure(file *ast.File, src source, stmt ast.Stmt) (analysis.TextEdit, bool) {
	var (
		start = src.Offset(stmt.Pos())
		end   = src.Offset(stmt.End())
		head  = src.lineStart(stmt.Pos())
		tail  = src.lineEnd(stmt.End())
	)
	if !src.within(head, start, end, tail) {
		return analysis.TextEdit{}, false
	}
	before := src.text[head:start]
	after := src.text[end:tail]
	if len(bytes.TrimSpace(before)) == 0 && len(bytes.TrimSpace(after)) == 0 {
		// The statement has its line to itself, and the line goes with it.
		stop := len(src.text)
		if tail < len(src.text) {
			stop = tail + 1
		}
		if commented(file, src.Pos(head), src.Pos(stop)) {
			return analysis.TextEdit{}, false
		}
		return analysis.TextEdit{Pos: src.Pos(head), End: src.Pos(stop)}, true
	}
	rest := bytes.TrimLeft(after, " \t")
	if len(rest) > 0 && rest[0] != ';' {
		return analysis.TextEdit{}, false
	}
	if len(rest) > 0 {
		// The semicolon separates the statement from the next one, and goes
		// with it, along with the spaces that follow.
		end++
		spaces := bytes.TrimLeft(after[1:], " \t")
		end += len(after) - 1 - len(spaces)
	} else {
		// Nothing follows on the line, so the spaces before the statement go
		// too, leaving no trailing whitespace behind.
		start = head + len(bytes.TrimRight(before, " \t"))
	}
	// A comment anywhere on the line belongs to the author, whether the edit
	// would delete it or leave it describing something else.
	if commented(file, src.Pos(start), src.Pos(tail)) {
		return analysis.TextEdit{}, false
	}
	return analysis.TextEdit{Pos: src.Pos(start), End: src.Pos(end)}, true
}

// commented reports whether a comment sits in the span a fix deletes, where it
// belongs to the author.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// fits reports whether the line the edit lands on stays within the width. A
// line already past the width fits: refusing the fix cannot shorten it, and
// linelen reports the line either way.
func fits(src source, pos, end token.Pos, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	var (
		head = src.lineStart(pos)
		from = src.Offset(pos)
		to   = src.Offset(end)
		tail = src.lineEnd(end)
	)
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the text, so the bounds are checked rather than
	// trusted.
	if !src.within(head, from, to, tail) {
		return false
	}
	if !linelen.CheckLine(src.text[head:tail]) {
		return true
	}
	var line []byte
	line = append(line, src.text[head:from]...)
	line = append(line, text...)
	line = append(line, src.text[to:tail]...)
	return linelen.CheckLine(line)
}
