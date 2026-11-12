package strictvar

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// checkPreamble enforces the preamble rule. Declarations set up ahead of use
// belong before the work that uses them, mirroring how package-level var
// blocks sit at the top of a file. The first declarations in a statement list
// can share one var block, and a var block anywhere else in the list
// dissolves into standalone declarations.
func (f *file) checkPreamble() {
	f.stmtLists(func(list []ast.Stmt) {
		if len(list) > 0 && f.claimed(list[0].Pos()) {
			return
		}
		f.checkRun(f.collectRun(list))
		f.checkStrayBlocks(list)
	})
}

// collectRun gathers the declaration units of the one run a statement list
// may group. Any number of statements that set up no variable may precede it,
// so a guard, a call, a const or a type declaration does not end the preamble
// before it starts.
//
// The run opens at the first statement that declares a variable, by var or by
// :=, and that statement decides the whole list: if it cannot join a block, no
// later run can either. An assignment further down therefore starts nothing,
// which is what keeps a block out of the middle of a function.
func (f *file) collectRun(list []ast.Stmt) (units []ast.Stmt) {
	for _, stmt := range list {
		if !declaresVar(stmt) {
			if len(units) == 0 {
				continue // a statement ahead of the run sets up no variable
			}
			return units
		}
		if len(units) > 0 {
			var (
				prev = units[len(units)-1]
				lo   = f.lineEnd(prev.End()) + 1
			)
			if f.hasComment(lo, f.lineStart(stmt.Pos())) {
				// A comment on its own line between
				// declarations keeps them apart, whatever
				// their form. A trailing comment stays on
				// the line it ends.
				return units
			}
		}
		if !f.declares(stmt) {
			// The first declaration decides the list: a
			// statement that cannot join a block ends the
			// preamble outright.
			return units
		}
		units = append(units, stmt)
	}
	return units
}

// declaresVar reports whether stmt declares a variable, by a := statement or
// a var declaration. Such a statement opens the one run a list may group,
// whether or not the block can hold it. A const or type declaration sets up no
// variable, so it is skipped ahead of the run and ends it like any other
// statement once the run has opened.
func declaresVar(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		return stmt.Tok == token.DEFINE
	case *ast.DeclStmt:
		gd, ok := stmt.Decl.(*ast.GenDecl)
		return ok && gd.Tok == token.VAR
	}
	return false
}

// declares reports whether stmt declares a variable in this scope: a var
// declaration without a doc comment, or a := statement the preamble rules
// admit.
func (f *file) declares(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		return f.preambleAssign(stmt)
	case *ast.DeclStmt:
		gd, ok := stmt.Decl.(*ast.GenDecl)
		return ok && gd.Tok == token.VAR && gd.Doc == nil
	}
	return false
}

// checkRun reports a run that can become one var block: two or more units. A
// run of same-type zero declarations stands down here: combining it into one
// spec is the tighter form.
func (f *file) checkRun(units []ast.Stmt) {
	if len(units) < 2 {
		return
	}
	if f.zeroRunStmts(units) {
		return
	}
	for _, stmt := range units {
		if ds, ok := stmt.(*ast.DeclStmt); ok {
			f.owned[ds.Decl.(*ast.GenDecl)] = struct{}{}
		}
	}
	first, last := units[0], units[len(units)-1]
	f.spans = append(f.spans, [2]token.Pos{first.Pos(), last.End()})
	f.report(first.Pos(), last.End(), fmt.Sprintf(
		"%d adjacent declarations can be grouped into a var block",
		len(units),
	),
		"Group the declarations into one block",
		f.preambleEdits(units),
	)
}

// preambleAssign reports whether a := statement can become a var spec: every
// left-hand name is a plain ident newly declared here, and the value is not a
// zero the zero-value rule owns, since that rule rewrites it to var first and
// the block forms on the next pass.
func (f *file) preambleAssign(assign *ast.AssignStmt) bool {
	if assign.Tok != token.DEFINE {
		return false
	}
	info := f.pass.TypesInfo
	for _, lhs := range assign.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			return false
		}
		obj := info.Defs[id]
		if id.Name != "_" && obj == nil {
			return false // redeclaration: = of an existing variable
		}
		if obj != nil && obj.Type().String() == "error" {
			// An error on the left means handling follows, so the call
			// stays beside its check.
			return false
		}
	}
	if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
		if _, zero := f.zeroRewrite("x", assign.Rhs[0]); zero {
			return false
		}
	}
	return true
}

// checkStrayBlocks reports var blocks outside the preamble of a statement
// list. A block is in the preamble when no declaration precedes it, so it
// opens the run the grouping rules build. Anywhere else its declarations read
// differently from the surrounding code, so each spec dissolves to the
// standalone form the other rules would choose.
func (f *file) checkStrayBlocks(list []ast.Stmt) {
	for i, stmt := range list {
		ds, ok := stmt.(*ast.DeclStmt)
		if !ok {
			continue
		}
		gd, ok := ds.Decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR || !gd.Lparen.IsValid() {
			continue
		}
		_, claimed := f.owned[gd]
		if claimed || f.preambleAnchored(list, i, gd) {
			continue
		}
		f.reportStrayBlock(gd)
	}
}

// preambleAnchored reports whether the block at index i opens the list, or
// holds two or more specs counting the declarations after it. A block of
// either shape stays: it is the form the grouping rules build, so dissolving
// it would only build it again.
func (f *file) preambleAnchored(list []ast.Stmt, i int, gd *ast.GenDecl) bool {
	if i == 0 {
		return true
	}
	n := len(gd.Specs)
	for _, stmt := range list[i+1:] {
		if !f.declares(stmt) {
			break
		}
		n++
	}
	return n >= 2
}

// reportStrayBlock reports a var block outside the preamble and suggests
// dissolving it into standalone declarations.
func (f *file) reportStrayBlock(gd *ast.GenDecl) {
	edits := f.dissolveEdits(gd)
	if edits != nil {
		f.owned[gd] = struct{}{}
	}
	// The diagnostic anchors on the first spec: the block header
	// vanishes under the fix, and the spec line survives it.
	pos, end := gd.Pos(), gd.End()
	if len(gd.Specs) > 0 {
		pos, end = gd.Specs[0].Pos(), gd.Specs[0].End()
	}
	msg := "var block outside the preamble can be dissolved into standalone declarations"
	f.report(pos, end, msg, "Dissolve the block", edits)
}

// dissolveEdits rebuilds a block as standalone declarations, one per spec.
// Doc and trailing comments ride with their specs, a doc-commented spec
// keeping the blank line the comment-spacing rule enforces. Blank lines
// between plain specs marked visual groups the block no longer holds, so
// they do not survive. A comment attached to nothing marks deliberate
// structure and yields no fix, as does a block comment documenting more than
// one spec.
func (f *file) dissolveEdits(gd *ast.GenDecl) []analysis.TextEdit {
	if !f.dissolvable(gd) {
		return nil
	}
	if len(gd.Specs) == 0 {
		pos, end, ok := f.deleteLineSpan(gd)
		if !ok {
			return nil
		}
		return []analysis.TextEdit{{Pos: pos, End: end}}
	}
	lines, kept, ok := f.dissolveLines(gd)
	if !ok || f.strayComments(gd.Lparen, gd.Rparen, kept) {
		return nil
	}
	text := f.dissolveText(lines, gd)
	if !parses(text) {
		return nil
	}
	return []analysis.TextEdit{{
		Pos:     gd.Pos(),
		End:     gd.End(),
		NewText: []byte(text),
	}}
}

// dissolvable reports whether a block can be written as standalone
// declarations: alone on its line, with nothing after it, and with at most
// one spec when it carries a doc comment.
func (f *file) dissolvable(gd *ast.GenDecl) bool {
	for _, b := range f.text(f.lineStart(gd.Pos()), gd.Pos()) {
		if b != ' ' && b != '\t' {
			return false
		}
	}
	if strings.TrimSpace(f.text(gd.End(), f.lineEnd(gd.End()))) != "" {
		return false
	}
	return gd.Doc == nil || len(gd.Specs) == 1
}

// dissolveLines returns the standalone line for each spec, and the source
// ranges whose comments carry over.
func (f *file) dissolveLines(gd *ast.GenDecl) (lines []string, kept [][2]token.Pos, ok bool) {
	for _, spec := range gd.Specs {
		vs, isSpec := spec.(*ast.ValueSpec)
		if !isSpec || !f.trailOK(vs.End()) {
			return nil, nil, false
		}
		inline := f.Line(vs.Pos()) == f.Line(gd.Lparen)
		multiline := f.Line(vs.End()) != f.Line(vs.Pos())
		if inline || multiline {
			return nil, nil, false
		}
		decl, ok := f.dissolveSpec(vs)
		if !ok {
			return nil, nil, false
		}
		if vs.Doc != nil {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			kept = append(kept, [2]token.Pos{vs.Doc.Pos(), vs.Doc.End()})
			for _, c := range vs.Doc.List {
				lines = append(lines, f.text(c.Pos(), c.End()))
			}
		}
		kept = append(kept, [2]token.Pos{vs.Pos(), f.lineEnd(vs.End())})
		if vs.Comment != nil {
			decl += " " + f.text(vs.Comment.Pos(), vs.Comment.End())
		}
		lines = append(lines, decl)
	}
	return lines, kept, true
}

// dissolveText joins the lines at the block's indent.
func (f *file) dissolveText(lines []string, gd *ast.GenDecl) string {
	var (
		sb     strings.Builder
		indent = f.indent(gd.Pos())
	)
	for i, l := range lines {
		if i > 0 {
			sb.WriteString("\n")
			if l != "" {
				sb.WriteString(indent)
			}
		}
		sb.WriteString(l)
	}
	return sb.String()
}

// dissolveSpec returns the standalone declaration replacing a block spec,
// classified the way the standalone rules would choose. Zero values keep
// var, initialized values take :=, and a spec whose type does work :=
// cannot express keeps var with its type.
func (f *file) dissolveSpec(vs *ast.ValueSpec) (string, bool) {
	var typ, vals string
	if vs.Type != nil {
		typ = " " + f.text(vs.Type.Pos(), vs.Type.End())
	}
	if len(vs.Values) > 0 {
		vals = f.text(vs.Values[0].Pos(), vs.Values[len(vs.Values)-1].End())
	}
	if len(vs.Values) == 0 {
		return "var " + names(vs) + typ, true
	}
	named := slices.ContainsFunc(vs.Names, func(n *ast.Ident) bool {
		return n.Name != "_"
	})
	if !named {
		// A := needs a new variable on its left, which _ is not.
		return "var " + names(vs) + typ + " = " + vals, true
	}
	if vs.Type == nil {
		if len(vs.Names) == 1 && len(vs.Values) == 1 {
			repl, zero := f.zeroRewrite(vs.Names[0].Name, vs.Values[0])
			if zero {
				return repl, true
			}
		}
		return names(vs) + " := " + vals, true
	}
	if len(vs.Names) != 1 || len(vs.Values) != 1 {
		return "var " + names(vs) + typ + " = " + vals, true
	}
	t := f.pass.TypesInfo.TypeOf(vs.Type)
	if t == nil {
		return "", false
	}
	if f.zeroForType(vs.Values[0], t) {
		return "var " + names(vs) + typ, true
	}
	if !f.typeFaithful(vs.Values[0], t) {
		return "var " + names(vs) + typ + " = " + vals, true
	}
	return names(vs) + " := " + vals, true
}

// preambleEdits merges the leading declarations into one var block.
func (f *file) preambleEdits(units []ast.Stmt) []analysis.TextEdit {
	var (
		kept  [][2]token.Pos
		lines []string
	)
	for _, stmt := range units {
		if !f.trailOK(stmt.End()) {
			return nil
		}
		switch node := stmt.(type) {
		case *ast.AssignStmt:
			if slices.ContainsFunc(node.Rhs, containsEmptyRef) {
				return nil
			}
			kept = append(kept, [2]token.Pos{
				stmt.Pos(), f.lineEnd(stmt.End()),
			})
			tail := f.lineText(span{node.TokPos + 2, node.End()})
			lines = append(
				lines,
				f.text(node.Pos(), node.TokPos)+"= "+
					strings.TrimLeft(tail, " \t"),
			)
		case *ast.DeclStmt:
			gd := node.Decl.(*ast.GenDecl)
			if gd.Lparen.IsValid() {
				// A comment trailing the var ( line has no spec to
				// ride: the header dissolves, so the fix stands down.
				blank := f.text(gd.Lparen+1, f.lineEnd(gd.Lparen))
				if strings.TrimSpace(blank) != "" {
					return nil
				}
			}
			for _, spec := range gd.Specs {
				_, ok := spec.(*ast.ValueSpec)
				if !ok || !f.trailOK(spec.End()) {
					return nil
				}
				kept = append(kept, [2]token.Pos{
					spec.Pos(),
					f.lineEnd(spec.End()),
				})
				lines = append(lines, f.lineText(spec))
			}
		}
	}
	first, last := units[0], units[len(units)-1]
	indent := f.indent(first.Pos())
	text := "var (\n" + blockText(indent, lines) + indent + ")"
	hi := f.lineEnd(last.End())
	if f.strayComments(first.Pos(), hi, kept) || !parses(text) {
		return nil
	}
	return []analysis.TextEdit{{
		Pos:     first.Pos(),
		End:     hi,
		NewText: []byte(text),
	}}
}
