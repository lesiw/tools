// Package sloppyreassign provides an analysis.Analyzer that reports an
// assignment in the init clause of an if, switch, or for statement whose
// variable is not used outside the statement.
//
// A short declaration confines the variable to the statement it is read
// in. The rule is adapted from go-critic's sloppyReassign.
//
// The fix declares the variable in the statement and deletes its local
// declaration where the init does not read it. The README lists the cases
// where no fix is offered.
package sloppyreassign

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "sloppyreassign",
	Doc:  "reports an assignment whose variable is not used outside its statement",
	Run:  run,
}

type assignment struct {
	stmt *ast.AssignStmt
	obj  *types.Var
}

// clause is a statement with an init clause, and the word the diagnostic
// uses for it.
type clause struct {
	stmt ast.Stmt
	init ast.Stmt
	kind string
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		writes := written(file)
		ast.Inspect(file, func(n ast.Node) bool {
			if c, ok := clauseOf(n); ok {
				check(pass, file, writes, c)
			}
			return true
		})
	}
	return nil, nil
}

// clauseOf returns the clause n opens, for the statements the rule covers.
func clauseOf(n ast.Node) (clause, bool) {
	switch n := n.(type) {
	case *ast.IfStmt:
		return clause{stmt: n, init: n.Init, kind: "if statement"}, true
	case *ast.SwitchStmt:
		return clause{stmt: n, init: n.Init, kind: "switch statement"}, true
	case *ast.TypeSwitchStmt:
		return clause{stmt: n, init: n.Init, kind: "switch statement"}, true
	case *ast.ForStmt:
		return clause{stmt: n, init: n.Init, kind: "for statement"}, true
	}
	return clause{}, false
}

// check reports c's init assignment when something inside c reads its value
// and nothing outside does.
func check(pass *analysis.Pass, file *ast.File, writes map[*ast.Ident]bool, c clause) {
	a := initAssign(pass, c.init)
	if a == nil {
		return
	}
	outside, inside, selfRead := reads(pass, file, writes, a.obj, c)
	// Nothing inside reads the value: the write is dead, and a declaration
	// in the clause would be a variable no one reads.
	if outside || !inside {
		return
	}
	if !sameType(pass, a) {
		return
	}
	// A naked return reads every named result.
	body := resultBody(pass, file, a.obj)
	if body != nil && nakedReturn(body) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     a.stmt.Pos(),
		End:     a.stmt.End(),
		Message: a.obj.Name() + " can be declared in this " + c.kind,
	}
	drop := body == nil && !selfRead
	if edits, ok := fixEdits(pass, file, a, drop); ok {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Declare " + a.obj.Name() + " in the " + c.kind,
			TextEdits: edits,
		}}
	}
	pass.Report(diag)
}

// fixEdits returns the edits that declare the variable in the clause. drop
// reports whether the rewrite leaves the variable unread, so its declaration
// goes with it; where the declaration cannot be deleted, the report stands
// without a fix.
func fixEdits(pass *analysis.Pass, file *ast.File, a *assignment, drop bool) ([]analysis.TextEdit, bool) {
	edits := []analysis.TextEdit{{
		Pos:     a.stmt.TokPos,
		End:     a.stmt.TokPos + 1,
		NewText: []byte(":="),
	}}
	if !fits(pass, a) {
		return nil, false
	}
	if !drop {
		return edits, true
	}
	edit, ok := declEdit(pass, file, a.obj)
	if !ok {
		return nil, false
	}
	return append(edits, edit), true
}

// fits reports whether the header line fits within the width once the
// assignment token grows by a byte. A line already past the width fits:
// refusing the fix cannot shorten it, and linelen reports the line either
// way.
func fits(pass *analysis.Pass, a *assignment) bool {
	tokFile := pass.Fset.File(a.stmt.TokPos)
	if tokFile == nil || linelen.Config.Len <= 0 {
		return true
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return false
	}
	at := tokFile.Offset(a.stmt.TokPos)
	from, to := lineStart(src, at), lineEnd(src, at)
	if from > len(src) || to > len(src) {
		return false
	}
	line := src[from:to]
	if !linelen.CheckLine(line) {
		return true
	}
	grown := make([]byte, 0, len(line)+1)
	grown = append(grown, line[:at-from]...)
	grown = append(grown, ':')
	grown = append(grown, line[at-from:]...)
	return linelen.CheckLine(grown)
}

// declEdit returns the edit removing obj from its var declaration, when the
// declaration can lose it without losing a value or a comment.
func declEdit(pass *analysis.Pass, file *ast.File, obj types.Object) (analysis.TextEdit, bool) {
	var (
		edit  analysis.TextEdit
		found bool
	)
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.DeclStmt)
		if !ok || found {
			return !found
		}
		decl, ok := stmt.Decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			return true
		}
		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Values) != 0 {
				continue
			}
			at := declaresName(pass, value, obj)
			if at < 0 {
				continue
			}
			if len(value.Names) > 1 {
				// A shared spec loses only the name; the rest stay.
				edit, found = namesEdit(pass, file, value, at)
				continue
			}
			if len(decl.Specs) > 1 {
				edit, found = lineEdit(pass, file, spec, true)
				continue
			}
			edit, found = lineEdit(pass, file, stmt, false)
		}
		return !found
	})
	return edit, found
}

// declaresName returns the index of obj among spec's names, or -1.
func declaresName(pass *analysis.Pass, spec *ast.ValueSpec, obj types.Object) int {
	for i, name := range spec.Names {
		if pass.TypesInfo.Defs[name] == obj {
			return i
		}
	}
	return -1
}

// namesEdit returns the edit removing one name from a spec, with the comma
// that joins it, or reports false when the spec declares only that name.
func namesEdit(pass *analysis.Pass, file *ast.File, spec *ast.ValueSpec, at int) (analysis.TextEdit, bool) {
	if len(spec.Names) < 2 {
		return analysis.TextEdit{}, false
	}
	var edit analysis.TextEdit
	if at == len(spec.Names)-1 {
		edit = analysis.TextEdit{
			Pos: spec.Names[at-1].End(),
			End: spec.Names[at].End(),
		}
	} else {
		edit = analysis.TextEdit{
			Pos: spec.Names[at].Pos(),
			End: spec.Names[at+1].Pos(),
		}
	}
	if commented(file, edit.Pos, edit.End) {
		return analysis.TextEdit{}, false
	}
	first := pass.Fset.Position(spec.Pos()).Line
	last := pass.Fset.Position(spec.End()).Line
	for line := first; line <= last; line++ {
		if commentedLine(pass, file, line) {
			return analysis.TextEdit{}, false
		}
	}
	if docComment(pass, file, first-1) {
		return analysis.TextEdit{}, false
	}
	return edit, true
}

// commented reports whether a comment lies in the text a fix would replace.
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

// lineEdit returns the edit removing the lines holding n, when n is the only
// thing on them and no line from the one above through the last carries a
// comment. absorb takes the blank lines beside n with it, which a group keeps
// when a spec between other specs goes.
func lineEdit(pass *analysis.Pass, file *ast.File, n ast.Node, absorb bool) (analysis.TextEdit, bool) {
	tokFile := pass.Fset.File(n.Pos())
	if tokFile == nil {
		return analysis.TextEdit{}, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return analysis.TextEdit{}, false
	}
	// A //line directive moves Position and not Offset, so the offsets index
	// the bytes that were parsed.
	start, end := tokFile.Offset(n.Pos()), tokFile.Offset(n.End())
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
	edit := analysis.TextEdit{
		Pos: tokFile.Pos(from),
		End: tokFile.Pos(to),
	}
	if absorb {
		edit = absorbBlank(src, tokFile, edit)
	}
	first, last := tokFile.Line(n.Pos()), tokFile.Line(n.End())
	for line := first; line <= last; line++ {
		if commentedLine(pass, file, line) {
			return analysis.TextEdit{}, false
		}
	}
	if docComment(pass, file, first-1) {
		return analysis.TextEdit{}, false
	}
	return edit, true
}

// absorbBlank widens edit over a blank line above or below it, which a group
// keeps when a spec between other specs goes.
func absorbBlank(src []byte, tokFile *token.File, edit analysis.TextEdit) analysis.TextEdit {
	from, to := tokFile.Offset(edit.Pos), tokFile.Offset(edit.End)
	if from > 0 && blank(src[lineStart(src, from-1):from-1]) {
		from = lineStart(src, from-1)
	}
	if to < len(src) && blank(src[to:lineEnd(src, to)]) {
		to = lineEnd(src, to) + 1
	}
	return analysis.TextEdit{Pos: tokFile.Pos(from), End: tokFile.Pos(to)}
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
func blank(src []byte) bool {
	for _, b := range src {
		if b != ' ' && b != '\t' && b != '\r' {
			return false
		}
	}
	return true
}

// docComment reports whether a comment starts line, which is what makes it a
// comment about the declaration below it rather than a trailing one.
func docComment(pass *analysis.Pass, file *ast.File, line int) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			pos := pass.Fset.Position(comment.Pos())
			if pos.Line != line {
				continue
			}
			src, err := pass.ReadFile(pos.Filename)
			if err != nil {
				return true
			}
			if blank(src[lineStart(src, pos.Offset):pos.Offset]) {
				return true
			}
		}
	}
	return false
}

// commentedLine reports whether a comment starts on line.
func commentedLine(pass *analysis.Pass, file *ast.File, line int) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if pass.Fset.Position(comment.Pos()).Line == line {
				return true
			}
		}
	}
	return false
}

// initAssign returns the assignment in init and the variable it writes, or nil
// when init is not a single plain assignment to a local variable.
func initAssign(pass *analysis.Pass, init ast.Stmt) *assignment {
	stmt, ok := init.(*ast.AssignStmt)
	if !ok || stmt.Tok != token.ASSIGN {
		return nil
	}
	if len(stmt.Lhs) != 1 {
		return nil
	}
	name, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok {
		return nil
	}
	// The variable's reads have to be in this file. A package-level
	// variable is read from everywhere, including another package that a
	// dot import names.
	obj, ok := pass.TypesInfo.ObjectOf(name).(*types.Var)
	if !ok || obj.Pkg() != pass.Pkg || obj.Parent() == pass.Pkg.Scope() {
		return nil
	}
	return &assignment{stmt: stmt, obj: obj}
}

// reads reports whether obj is used outside c, whether it is read inside c
// after the init clause, and whether the clause itself reads it. A
// declaration in the clause is in scope from the end of it: a read after it
// sees the declared variable, and a read inside it still sees the old one.
//
// A use from a function literal inside c, or through the variable's address,
// happens when the deferred or stored call runs, which can be after the
// statement. Those count as uses outside.
func reads(pass *analysis.Pass, file *ast.File, writes map[*ast.Ident]bool, obj types.Object, c clause) (outside, inside, selfRead bool) {
	lits := literals(c.stmt)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.UnaryExpr:
			id, ok := ast.Unparen(n.X).(*ast.Ident)
			if ok && n.Op == token.AND && pass.TypesInfo.Uses[id] == obj {
				outside = true
			}
		case *ast.Ident:
			if pass.TypesInfo.Uses[n] != obj {
				return true
			}
			if n.Pos() < c.stmt.Pos() || n.Pos() > c.stmt.End() {
				outside = true
			}
			if inLiteral(n.Pos(), lits) {
				outside = true
			}
			if writes[n] {
				return true
			}
			if n.Pos() >= c.init.End() {
				inside = true
			} else {
				selfRead = true
			}
		}
		return true
	})
	return outside, inside, selfRead
}

// written collects the identifiers a plain assignment writes. x++ and x += y
// also read x, so they are not writes for this purpose.
func written(file *ast.File) map[*ast.Ident]bool {
	out := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.ASSIGN {
				return true
			}
			for _, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					out[id] = true
				}
			}
		case *ast.RangeStmt:
			for _, expr := range []ast.Expr{n.Key, n.Value} {
				if id, ok := expr.(*ast.Ident); ok {
					out[id] = true
				}
			}
		}
		return true
	})
	return out
}

// literals returns the outermost function literals inside n.
func literals(n ast.Node) []*ast.FuncLit {
	var lits []*ast.FuncLit
	ast.Inspect(n, func(n ast.Node) bool {
		lit, ok := n.(*ast.FuncLit)
		if ok {
			lits = append(lits, lit)
		}
		return !ok
	})
	return lits
}

func inLiteral(pos token.Pos, lits []*ast.FuncLit) bool {
	for _, lit := range lits {
		if pos >= lit.Pos() && pos <= lit.End() {
			return true
		}
	}
	return false
}

// sameType reports whether the assigned value has the variable's type. Where
// it does not, a declaration would give the variable the value's type: a typed
// nil stored in an interface would stop comparing equal to nil. A constant is
// left out because a declaration gives it its default type: declaring 1 makes
// a float64 variable an int.
func sameType(pass *analysis.Pass, a *assignment) bool {
	tv, ok := pass.TypesInfo.Types[a.stmt.Rhs[0]]
	return ok && tv.Value == nil && types.Identical(tv.Type, a.obj.Type())
}

// resultBody returns the body of the function declaring obj as a named
// result, or nil. A named result may go unassigned, so the rewrite can
// leave it unread. Any other declaration has to stay read, or the
// rewrite leaves a variable the compiler or another analyzer reports.
func resultBody(pass *analysis.Pass, file *ast.File, obj types.Object) *ast.BlockStmt {
	var (
		body *ast.BlockStmt
		set  = func(fn *ast.FuncType, b *ast.BlockStmt) {
			if declares(pass, fn.Results, obj) {
				body = b
			}
		}
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			set(fn.Type, fn.Body)
		case *ast.FuncLit:
			set(fn.Type, fn.Body)
		}
		return body == nil
	})
	return body
}

func declares(pass *analysis.Pass, list *ast.FieldList, obj types.Object) bool {
	if list == nil {
		return false
	}
	for _, field := range list.List {
		for _, name := range field.Names {
			if pass.TypesInfo.Defs[name] == obj {
				return true
			}
		}
	}
	return false
}

// nakedReturn reports whether body has a return statement with no results.
func nakedReturn(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) == 0 {
			found = true
		}
		return !found
	})
	return found
}
