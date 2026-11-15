// Package modifiesparam provides an analysis.Analyzer that reports a dead
// write to a parameter.
//
// The calls section of the spec says new storage is allocated for a function's
// variables, "which includes its parameters and results", and that the
// arguments are assigned to them. A parameter is the callee's copy, so the
// caller does not see a write to it, and the spec gives a method's receiver an
// extra parameter section of the same kind. A write that no read follows does
// nothing. A result reaches the caller, so the rule keeps to parameters.
//
// A parameter the body writes and then reads is a local copy used as a scratch
// variable. The rule stands down there. The README states which reads save a
// write, which writes are left out, and what the fix removes.
//
// An increment, a write whose right side does no work, and a range target
// carry a fix that removes the write, except where the edit would take a
// comment or another statement on the line with it. A write that shares its
// statement with a target the function reads goes with the right side beside
// it, or becomes a blank where that side does work. Otherwise the write is
// reported without a fix, since deleting it would drop the work it does.
//
// The rule is adapted from revive's modifies-parameter, which reports every
// assignment to a name in a function declaration's parameter list, without
// resolving the name.
package modifiesparam

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "modifiesparam",
	Doc:  "reports a dead write to a parameter",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch fn := n.(type) {
			case *ast.FuncDecl:
				if fn.Body != nil {
					vars := copies(pass, fn)
					check(pass, file, src, vars, fn.Body)
				}
			case *ast.FuncLit:
				vars := paramVars(pass, fn.Type.Params)
				check(pass, file, src, vars, fn.Body)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the writes to each of vars that nothing in body reads.
func check(pass *analysis.Pass, file *ast.File, src []byte, vars []*types.Var, body *ast.BlockStmt) {
	if len(vars) == 0 {
		return
	}
	var (
		jump   = jumps(body)
		posts  = postLoops(body)
		writes []write
	)
	for _, param := range vars {
		f := &fn{pass: pass, body: body, posts: posts, param: param}
		f.scan()
		if len(f.writes) == 0 || jump || f.escapes() {
			continue
		}
		writes = append(writes, f.dead()...)
	}
	report(pass, file, src, body, writes)
}

// copies returns the values a function body receives as its own copies: the
// receiver of a method, which the spec specifies as an extra parameter
// section, and the parameters. A pointer receiver is a copy of a pointer, so a
// write to it is dead too, while a write through it reaches the caller.
func copies(pass *analysis.Pass, fn *ast.FuncDecl) []*types.Var {
	out := paramVars(pass, fn.Recv)
	return append(out, paramVars(pass, fn.Type.Params)...)
}

// paramVars returns the variables list declares. A function declaration has no
// receiver field list.
func paramVars(pass *analysis.Pass, list *ast.FieldList) (vars []*types.Var) {
	if list == nil {
		return nil
	}
	for _, field := range list.List {
		for _, name := range field.Names {
			if v, ok := pass.TypesInfo.Defs[name].(*types.Var); ok {
				vars = append(vars, v)
			}
		}
	}
	return vars
}

// fn is one function body, the parameter it is checked for, and the writes to
// that parameter.
type fn struct {
	pass   *analysis.Pass
	body   *ast.BlockStmt
	posts  map[ast.Stmt]ast.Stmt // the loop each post statement belongs to
	param  *types.Var
	writes []write
}

// write is one assignment to the parameter.
type write struct {
	param   *types.Var
	target  ast.Expr   // the parameter, as the assignment writes it
	targets []ast.Expr // every target of the writing statement
	stmt    ast.Stmt   // the statement doing the writing, nil for a range
	loop    ast.Stmt   // the for or range statement whose clause writes
}

// end returns the position the write finishes at, which a read has to pass to
// read the value it wrote.
func (w write) end() token.Pos {
	if w.stmt == nil {
		return w.target.End()
	}
	return w.stmt.End()
}

// value returns the expression the write assigns, or nil for an increment, a
// range, or a statement that assigns one value to several targets.
func (w write) value() ast.Expr {
	assign, ok := w.stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != len(assign.Rhs) {
		return nil
	}
	for i, lhs := range assign.Lhs {
		if lhs == w.target {
			return assign.Rhs[i]
		}
	}
	return nil
}

// scan records the assignments that write the parameter.
func (f *fn) scan() {
	ast.Inspect(f.body, func(n ast.Node) bool {
		var (
			stmt, loop ast.Stmt
			targets    []ast.Expr
		)
		switch n := n.(type) {
		case *ast.AssignStmt:
			stmt, targets = n, n.Lhs
		case *ast.IncDecStmt:
			stmt, targets = n, []ast.Expr{n.X}
		case *ast.RangeStmt:
			// A range clause writes before each pass of its body, so the
			// statement covering the body is not the one doing the write.
			loop = n
			if n.Key != nil {
				targets = append(targets, n.Key)
			}
			if n.Value != nil {
				targets = append(targets, n.Value)
			}
		default:
			return true
		}
		for _, target := range targets {
			if f.isParam(target) {
				f.writes = append(f.writes, write{
					param:   f.param,
					target:  target,
					targets: targets,
					stmt:    stmt,
					loop:    loop,
				})
			}
		}
		return true
	})
}

// isParam reports whether expr is the parameter itself. A field written
// through a pointer parameter is another value, which the caller does see.
func (f *fn) isParam(expr ast.Expr) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && f.pass.TypesInfo.ObjectOf(id) == f.param
}

// place reports whether expr names memory inside the parameter's copy: the
// parameter, a field of it, or an element of an array it holds. A pointer the
// copy holds points elsewhere.
func (f *fn) place(expr ast.Expr) bool {
	switch x := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return f.isParam(x)
	case *ast.SelectorExpr:
		selection, ok := f.pass.TypesInfo.Selections[x]
		return ok && selection.Kind() == types.FieldVal &&
			!selection.Indirect() && f.place(x.X)
	case *ast.IndexExpr:
		return astexpr.IsArray(f.pass.TypesInfo.TypeOf(x.X)) && f.place(x.X)
	}
	return false
}

// dead returns the writes that nothing reads.
func (f *fn) dead() (dead []write) {
	for _, w := range f.writes {
		if f.observed(w) {
			continue
		}
		if w.loop == nil {
			w.loop = f.posts[w.stmt]
		}
		dead = append(dead, w)
	}
	return dead
}

// observed reports whether anything sees the write: a read of the value it
// leaves, the collection a nil assignment allows, or a loop or func literal
// that runs again.
func (f *fn) observed(w write) bool {
	if f.release(w) {
		return true
	}
	return after(f.reads(f.body, w.stmt), w.end()) || f.delayed(w)
}

// reads returns where the reads of the parameter end, leaving out the
// assignment targets that write it.
func (f *fn) reads(node ast.Node, skip ast.Stmt) (ends []token.Pos) {
	ast.Inspect(node, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || f.pass.TypesInfo.Uses[id] != f.param {
			return true
		}
		if holds(skip, id.Pos()) {
			return true
		}
		if f.written(id.Pos()) && !f.selfRead(id.Pos()) {
			return true
		}
		ends = append(ends, id.End())
		return true
	})
	return ends
}

// written reports whether pos is inside an assignment target.
func (f *fn) written(pos token.Pos) bool {
	for _, w := range f.writes {
		if holds(w.target, pos) {
			return true
		}
	}
	return false
}

// selfRead reports whether pos is a target that its own statement reads, as an
// increment or a compound assignment does. A loop that contains one reads the
// value it wrote on its next pass.
func (f *fn) selfRead(pos token.Pos) bool {
	for _, w := range f.writes {
		if holds(w.target, pos) && astexpr.SelfReads(w.stmt) {
			return true
		}
	}
	return false
}

// release reports whether the write drops a reference by assigning the
// predeclared nil. Nothing reads the value, but the assignment may end the
// reference's life before the function returns, which is a purpose no read
// shows.
func (f *fn) release(w write) bool {
	value := w.value()
	if value == nil {
		return false
	}
	if tv, ok := f.pass.TypesInfo.Types[value]; ok && tv.IsNil() {
		return true
	}
	// A conversion of nil drops the reference the same way. A call of any
	// other kind does not.
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	if tv, ok := f.pass.TypesInfo.Types[call.Fun]; !ok || !tv.IsType() {
		return false
	}
	tv, ok := f.pass.TypesInfo.Types[call.Args[0]]
	return ok && tv.IsNil()
}

// delayed reports whether a loop that contains the write, or a func literal
// anywhere in the body, reads the parameter. A loop reads the value again on
// its next pass, and the tool cannot tell when a func literal runs, so a read
// in either saves every write to that parameter in the body.
func (f *fn) delayed(w write) bool {
	var found bool
	ast.Inspect(f.body, func(n ast.Node) bool {
		if found {
			return false
		}
		var (
			governed = true
			skip     ast.Stmt
		)
		switch n := n.(type) {
		case *ast.FuncLit:
			// A literal runs once, so the reads its writing statement makes
			// are its own.
			skip = w.stmt
		case *ast.ForStmt, *ast.RangeStmt:
			governed = holds(n, w.target.Pos())
		default:
			governed = false
		}
		if governed {
			found = len(f.reads(n, skip)) > 0
		}
		return !found
	})
	return found
}

// escapes reports whether the body hands out the parameter's address, which a
// write to the parameter can be read through.
func (f *fn) escapes() bool {
	var found bool
	ast.Inspect(f.body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.UnaryExpr:
			found = n.Op == token.AND && f.place(n.X)
		case *ast.SliceExpr:
			// A slice of an array points into the copy.
			typ := f.pass.TypesInfo.TypeOf(n.X)
			found = astexpr.IsArray(typ) && f.place(n.X)
		case *ast.SelectorExpr:
			// A pointer-receiver method takes the copy's address.
			selection, ok := f.pass.TypesInfo.Selections[n]
			if !ok || selection.Kind() != types.MethodVal {
				return true
			}
			sig := selection.Obj().Type().(*types.Signature)
			if _, ptr := sig.Recv().Type().(*types.Pointer); ptr {
				found = f.place(n.X)
			}
		}
		return !found
	})
	return found
}

// postLoops returns the loop each post statement belongs to.
func postLoops(body *ast.BlockStmt) map[ast.Stmt]ast.Stmt {
	out := make(map[ast.Stmt]ast.Stmt)
	ast.Inspect(body, func(n ast.Node) bool {
		loop, ok := n.(*ast.ForStmt)
		if !ok {
			return true
		}
		if loop.Post != nil {
			out[loop.Post] = loop
		}
		return true
	})
	return out
}

// jumps reports whether body contains a goto, which can carry execution back
// over a write to a read.
func jumps(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			// A label belongs to the function that declares it.
			return false
		case *ast.BranchStmt:
			found = n.Tok == token.GOTO
		}
		return !found
	})
	return found
}

// report names each dead write, and offers the edit that removes it.
func report(pass *analysis.Pass, file *ast.File, src []byte, body *ast.BlockStmt, writes []write) {
	discarded := make(map[ast.Expr]bool, len(writes))
	for _, w := range writes {
		discarded[w.target] = true
	}
	for _, w := range writes {
		diag := analysis.Diagnostic{
			Pos:     w.target.Pos(),
			End:     w.target.End(),
			Message: "dead write to parameter " + w.param.Name(),
		}
		if fix, ok := removal(pass, file, src, body, discarded, w); ok {
			diag.SuggestedFixes = []analysis.SuggestedFix{fix}
		}
		pass.Report(diag)
	}
}

// allDead reports whether every target is one of the dead writes. The blank
// identifier assigns nothing, so it does not count.
func allDead(discarded map[ast.Expr]bool, targets []ast.Expr) bool {
	for _, target := range targets {
		if id, ok := ast.Unparen(target).(*ast.Ident); ok && id.Name == "_" {
			continue
		}
		if !discarded[target] {
			return false
		}
	}
	return true
}

// removal returns the fix that takes the write out of the program, and reports
// false where the tool cannot place the edit.
func removal(pass *analysis.Pass, file *ast.File, src []byte, body *ast.BlockStmt, discarded map[ast.Expr]bool, w write) (analysis.SuggestedFix, bool) {
	if !allDead(discarded, w.targets) {
		if edits, ok := dropPairs(pass, body, discarded, w); ok {
			return fix("Delete the write", edits), true
		}
		if edits, ok := dropValue(discarded, w); ok {
			return fix("Delete the write", edits), true
		}
		// Another target of the statement is read, so the dead one becomes a
		// blank and the statement stays. The right side runs either way.
		edits := blankTargets(discarded, w.targets)
		return fix("Replace the target with _", edits), true
	}
	edits, ok := removeStatement(pass, file, src, body, w)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	return fix("Delete the write", edits), true
}

// fix returns the suggested fix that names one edit.
func fix(message string, edits []analysis.TextEdit) analysis.SuggestedFix {
	return analysis.SuggestedFix{Message: message, TextEdits: edits}
}

// removeStatement returns the edits that take the write out of the program,
// and reports false where the tool cannot place them or the right side may do
// work that deleting the write would drop.
func removeStatement(pass *analysis.Pass, file *ast.File, src []byte, body *ast.BlockStmt, w write) ([]analysis.TextEdit, bool) {
	if !inert(pass, body, w) {
		return nil, false
	}
	switch loop := w.loop.(type) {
	case *ast.ForStmt:
		return postEdits(file, loop)
	case *ast.RangeStmt:
		return rangeEdits(file, loop)
	}
	edit, ok := lineEdit(pass, file, src, w.stmt)
	if !ok {
		return nil, false
	}
	return []analysis.TextEdit{edit}, true
}

// dropPairs returns the edits that take dead targets out of a statement that
// assigns one value per target, with the values beside them, which is how
// half a swap or the tail of a chain goes. It reports false where a value
// cannot go with its target.
func dropPairs(pass *analysis.Pass, body *ast.BlockStmt, discarded map[ast.Expr]bool, w write) ([]analysis.TextEdit, bool) {
	assign, ok := w.stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != len(assign.Rhs) {
		return nil, false
	}
	var edits []analysis.TextEdit
	first, last := -1, -1
	flush := func() {
		if first < 0 {
			return
		}
		edits = append(edits,
			dropped(assign.Lhs, first, last),
			dropped(assign.Rhs, first, last),
		)
		first, last = -1, -1
	}
	for i, lhs := range assign.Lhs {
		if !discarded[lhs] {
			flush()
			continue
		}
		if !droppable(pass, body, assign.Rhs[i]) {
			return nil, false
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	flush()
	return edits, len(edits) > 0
}

// dropped returns the edit that removes list[first] through list[last], with
// the comma that joins it to a neighbor.
func dropped(list []ast.Expr, first, last int) analysis.TextEdit {
	if last+1 < len(list) {
		return analysis.TextEdit{
			Pos: list[first].Pos(),
			End: list[last+1].Pos(),
		}
	}
	return analysis.TextEdit{
		Pos: list[first-1].End(),
		End: list[last].End(),
	}
}

// droppable reports whether a value can go with its dead target: it does no
// work, and every name it holds keeps another use, so no variable loses its
// last read. An index or a selector can panic, and a call can work, so those
// stay.
func droppable(pass *analysis.Pass, body *ast.BlockStmt, value ast.Expr) bool {
	id, ok := ast.Unparen(value).(*ast.Ident)
	if !ok {
		tv, found := pass.TypesInfo.Types[value]
		return found && tv.Value != nil
	}
	obj := pass.TypesInfo.ObjectOf(id)
	var uses int
	ast.Inspect(body, func(n ast.Node) bool {
		other, ok := n.(*ast.Ident)
		if ok && pass.TypesInfo.Uses[other] == obj {
			uses++
		}
		return true
	})
	return uses > 1
}

// dropValue returns the edit that takes a dead value out of a range clause,
// which then assigns its key alone.
func dropValue(discarded map[ast.Expr]bool, w write) ([]analysis.TextEdit, bool) {
	loop, ok := w.loop.(*ast.RangeStmt)
	if !ok || loop.Key == nil || loop.Value == nil {
		return nil, false
	}
	if !discarded[loop.Value] || discarded[loop.Key] {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos: loop.Key.End(),
		End: loop.Value.End(),
	}}, true
}

// blankTargets returns the edits that write the blank identifier over each
// dead target.
func blankTargets(discarded map[ast.Expr]bool, targets []ast.Expr) (edits []analysis.TextEdit) {
	for _, target := range targets {
		if discarded[target] {
			edits = append(edits, analysis.TextEdit{
				Pos:     target.Pos(),
				End:     target.End(),
				NewText: []byte("_"),
			})
		}
	}
	return edits
}

// inert reports whether the write does nothing but assign. An increment and a
// range clause have no right side, and a right side that would do no work and
// keep its names can go with the write.
func inert(pass *analysis.Pass, body *ast.BlockStmt, w write) bool {
	switch stmt := w.stmt.(type) {
	case nil:
		return true
	case *ast.IncDecStmt:
		return true
	case *ast.AssignStmt:
		for _, value := range stmt.Rhs {
			if !droppable(pass, body, value) {
				return false
			}
		}
		return true
	}
	return false
}

// postEdits returns the edits that take a dead post clause out of a for
// header. A for clause of a condition alone holds neither the empty init
// before it nor a post after it, and a clause of nothing but a post becomes an
// endless loop.
func postEdits(file *ast.File, loop *ast.ForStmt) ([]analysis.TextEdit, bool) {
	if loop.Init != nil {
		return nil, false
	}
	// A comment anywhere in the header would go with the rewrite.
	if commented(file, loop.For, loop.Body.Lbrace) {
		return nil, false
	}
	if loop.Cond == nil {
		return []analysis.TextEdit{{
			Pos:     loop.For + 3,
			End:     loop.Body.Lbrace,
			NewText: []byte(" "),
		}}, true
	}
	return []analysis.TextEdit{
		{Pos: loop.For + 3, End: loop.Cond.Pos(), NewText: []byte(" ")},
		{Pos: loop.Cond.End(), End: loop.Body.Lbrace, NewText: []byte(" ")},
	}, true
}

// rangeEdits returns the edits that take a dead target out of a range clause.
// The loop keeps its range and its body.
func rangeEdits(file *ast.File, loop *ast.RangeStmt) ([]analysis.TextEdit, bool) {
	if commented(file, loop.For, loop.Range) {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos:     loop.For + 3,
		End:     loop.Range,
		NewText: []byte(" "),
	}}, true
}

// lineEdit returns the edit removing the lines holding stmt, when stmt is the
// only thing on them and carries no comment of its own.
func lineEdit(pass *analysis.Pass, file *ast.File, src []byte, stmt ast.Stmt) (analysis.TextEdit, bool) {
	if commented(file, stmt.Pos(), stmt.End()) {
		return analysis.TextEdit{}, false
	}
	if documented(pass, file, src, stmt) {
		return analysis.TextEdit{}, false
	}
	tokFile := pass.Fset.File(stmt.Pos())
	if tokFile == nil {
		return analysis.TextEdit{}, false
	}
	start, end := tokFile.Offset(stmt.Pos()), tokFile.Offset(stmt.End())
	if start > len(src) || end > len(src) {
		return analysis.TextEdit{}, false
	}
	from, to := lineStart(src, start), lineEnd(src, end)
	if !blank(src[from:start]) || !blank(src[end:to]) {
		return analysis.TextEdit{}, false
	}
	if to < len(src) {
		to++
	}
	edit := analysis.TextEdit{Pos: tokFile.Pos(from), End: tokFile.Pos(to)}
	return edit, true
}

// documented reports whether a comment starts the line above stmt, which is a
// comment about the statement rather than a trailing one.
func documented(pass *analysis.Pass, file *ast.File, src []byte, stmt ast.Stmt) bool {
	tokFile := pass.Fset.File(stmt.Pos())
	if tokFile == nil {
		return false
	}
	above := tokFile.Line(stmt.Pos()) - 1
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if tokFile.Line(comment.Pos()) != above {
				continue
			}
			off := tokFile.Offset(comment.Pos())
			if blank(src[lineStart(src, off):off]) {
				return true
			}
		}
	}
	return false
}

// commented reports whether a comment lies in the text an edit would replace.
func commented(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() >= from && comment.Pos() < to {
				return true
			}
		}
	}
	return false
}

// lineStart returns the offset of the first byte on off's line.
func lineStart(src []byte, off int) int {
	for i := off - 1; i >= 0; i-- {
		if src[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// lineEnd returns the offset of the newline that ends off's line.
func lineEnd(src []byte, off int) int {
	for i := off; i < len(src); i++ {
		if src[i] == '\n' {
			return i
		}
	}
	return len(src)
}

// blank reports whether src is indentation alone.
func blank(src []byte) bool { return len(bytes.TrimSpace(src)) == 0 }

// after reports whether one of ends is past pos.
func after(ends []token.Pos, pos token.Pos) bool {
	for _, end := range ends {
		if end > pos {
			return true
		}
	}
	return false
}

// holds reports whether pos lies within node. A nil node holds nothing.
func holds(node ast.Node, pos token.Pos) bool {
	return node != nil && node.Pos() <= pos && pos < node.End()
}
