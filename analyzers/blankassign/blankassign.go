// Package blankassign provides an analysis.Analyzer that reports a statement
// that assigns the value of a name or a basic literal to the blank identifier
// when the assignment has no effect.
//
// Effective Go says the blank identifier "can be assigned or declared with any
// value of any type, with the value discarded harmlessly"
// (https://go.dev/doc/effective_go#blank). Discarding the value is not an
// effect, and a statement like _ = x does nothing but read x. When the code
// uses x again with no write in between, the assignment has no effect:
// deleting the statement leaves the program unchanged. A use on the other side
// of a write does not count, since it sees a different value; the blank
// assignment may be the only read that keeps a benchmark's computation from
// being eliminated. A use in another blank assignment the fix can delete does
// not count either, since a fix that deleted both could leave a local variable
// or an import unused. A name with no other use is left alone as well: the
// assignment may be there for a linter. A basic literal or a predeclared
// constant is reported without another use, since no variable's value is
// involved.
//
// Only a numeric or boolean name declared inside a function is reported. A
// value of any other type can hold a reference, whose lifetime a use can
// extend, so deleting the read could let a finalizer run earlier; it can also
// be written through a selector, an index, or a copied slice that the analysis
// does not track. A package-level variable is left alone because its reads and
// writes can come from other functions. An assignment of a name whose address
// is taken is left alone for the same reason.
//
// The right-hand side is an identifier, a basic literal, or a name qualified
// by a package. A call, a receive, an index expression, a dereference, and a
// field selection are left alone: evaluating one can have an effect or panic.
// An assignment with more than one target or value is outside the rule, as is
// a declaration of the blank identifier. A labeled statement is outside the
// rule too, because deleting it would leave the label without a statement.
//
// A package-qualified name is also left alone when the file uses its import
// nowhere else: deleting the statement would leave the import unused, and the
// file would not compile. A name from a dot import is left alone too, since
// its import is not visible from the name alone.
//
// Safe fix: delete the statement, and its lines when it occupies them alone. A
// comment inside the statement or on one of its lines, or a directive on the
// line before it, makes the fix unavailable, since the edit would lose or
// detach it.
//
// The rule is inspired by staticcheck's S1005, which drops unnecessary blank
// identifiers in range clauses, multiple assignments, and channel receives.
// This rule reports the _ = name and _ = literal forms that S1005 does not
// cover.
package blankassign

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "blankassign",
	Doc:  "reports an assignment to the blank identifier that has no effect",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	var (
		labels = labeled(pass)
		found  = accesses(pass, labels)
		check  = checker{
			pass:     pass,
			access:   found,
			labeled:  labels,
			taken:    taken(pass),
			captured: captured(pass),
		}
	)
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		check.src = src
		check.comments = file.Comments
		check.file(file)
	}
	return nil, nil
}

// checker reports the blank assignments in one package.
type checker struct {
	pass     *analysis.Pass
	access   map[types.Object]*live       // reads and writes
	labeled  map[*ast.AssignStmt]struct{} // labeled assignments
	taken    map[types.Object]struct{}    // address-taken objects
	captured map[types.Object]struct{}    // objects used in a function literal
	src      []byte                       // file under inspection
	comments []*ast.CommentGroup          // comments of that file
}

// live records the positions where an object is read and written.
type live struct {
	reads  []token.Pos
	writes []token.Pos
}

// file checks the assignments in file.
func (c *checker) file(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			c.check(assign)
		}
		return true
	})
}

// check reports assign when it assigns a value to the blank identifier and
// deleting the statement changes nothing.
func (c *checker) check(assign *ast.AssignStmt) {
	if !blankOf(assign) {
		return
	}
	if _, ok := c.labeled[assign]; ok {
		return
	}
	if !c.value(ast.Unparen(assign.Rhs[0])) {
		return
	}
	diagnostic := analysis.Diagnostic{
		Pos:     assign.Pos(),
		End:     assign.End(),
		Message: "assignment to the blank identifier has no effect",
	}
	if fixable(c.pass, assign, c.labeled, c.comments) {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Delete the assignment",
			TextEdits: []analysis.TextEdit{c.removal(assign)},
		}}
	}
	c.pass.Report(diagnostic)
}

// value reports whether expr is a value the rule can report.
func (c *checker) value(expr ast.Expr) bool {
	if !candidate(c.pass, expr) {
		return false
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		obj := c.pass.TypesInfo.Uses[expr]
		if _, ok := obj.(*types.Const); ok && obj.Parent() == types.Universe {
			return true
		}
		return c.used(expr)
	case *ast.SelectorExpr:
		pkg, ok := ast.Unparen(expr.X).(*ast.Ident)
		if !ok {
			return false
		}
		return c.used(pkg) && c.used(expr.Sel)
	}
	return true
}

// candidate reports whether expr has one of the shapes the rule reports: an
// identifier, a basic literal, or a name qualified by a package.
func candidate(pass *analysis.Pass, expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		// A name from a dot import is not matched; its file's import is not
		// visible from the object alone.
		obj := pass.TypesInfo.Uses[expr]
		return obj == nil || obj.Pkg() == nil || obj.Pkg() == pass.Pkg
	case *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		pkg, ok := ast.Unparen(expr.X).(*ast.Ident)
		if !ok {
			return false
		}
		_, ok = pass.TypesInfo.Uses[pkg].(*types.PkgName)
		return ok
	}
	return false
}

// used reports whether id may be reported: a package name needs another use of
// the import, and any other name needs a reference-free value that another use
// observes and no address takes.
func (c *checker) used(id *ast.Ident) bool {
	obj := c.pass.TypesInfo.Uses[id]
	if _, ok := obj.(*types.PkgName); ok {
		return otherUse(c.access[obj], id.Pos())
	}
	if v, ok := obj.(*types.Var); ok {
		if v.Pkg() != nil && v.Parent() == v.Pkg().Scope() {
			return false
		}
	}
	if _, ok := c.taken[obj]; ok {
		return false
	}
	if _, ok := c.captured[obj]; ok {
		return false
	}
	if !scalar(c.pass.TypesInfo.TypeOf(id)) {
		return false
	}
	access, ok := c.access[obj]
	return ok && otherRead(access, id.Pos())
}

// otherUse reports whether the file uses the import at a position other than
// pos. An import has no value, so a read anywhere in the file counts.
func otherUse(access *live, pos token.Pos) bool {
	if access == nil {
		return false
	}
	for _, read := range access.reads {
		if read != pos {
			return true
		}
	}
	return false
}

// otherRead reports whether a use of the object, other than the one at pos,
// lies in the same stretch between writes. A use on the other side of a write
// sees a different value.
func otherRead(access *live, pos token.Pos) bool {
	var previous, next token.Pos
	for _, write := range access.writes {
		if write < pos && write > previous {
			previous = write
		}
		if write > pos && (next == token.NoPos || write < next) {
			next = write
		}
	}
	for _, read := range access.reads {
		if read == pos {
			continue
		}
		if read > previous && (next == token.NoPos || read < next) {
			return true
		}
	}
	return false
}

// scalar reports whether t is a numeric or boolean type. A scalar value has
// no referenced object to keep alive, and a local scalar is written only by
// the direct assignments that the analysis records.
func scalar(t types.Type) bool {
	if t == nil {
		return false
	}
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	const numbers = types.IsInteger | types.IsFloat | types.IsComplex
	return basic.Info()&(numbers|types.IsBoolean) != 0
}

// commented reports whether deleting assign would lose or detach a comment: a
// comment sits inside the statement or on one of its lines, or a directive
// sits on the line before it.
func commented(assign *ast.AssignStmt, comments []*ast.CommentGroup, tokFile *token.File) bool {
	first, last := tokFile.Line(assign.Pos()), tokFile.Line(assign.End())
	for _, group := range comments {
		for _, comment := range group.List {
			line := tokFile.Line(comment.Pos())
			if line >= first && line <= last {
				return true
			}
			if line == first-1 && isDirective(comment.Text) {
				return true
			}
		}
	}
	return false
}

// isDirective reports whether text is a compiler or tool directive.
func isDirective(text string) bool {
	return strings.HasPrefix(text, "//go:") ||
		strings.HasPrefix(text, "//line ") ||
		strings.HasPrefix(text, "//ignore") ||
		strings.HasPrefix(text, "//export")
}

// removal returns the edit that deletes assign. When the statement is alone on
// its lines, the fix deletes them; otherwise it deletes only the statement.
func (c *checker) removal(assign *ast.AssignStmt) analysis.TextEdit {
	var (
		tokFile    = c.pass.Fset.File(assign.Pos())
		start, end = tokFile.Offset(assign.Pos()), tokFile.Offset(assign.End())
	)
	if source.Alone(c.src, start, end) {
		return analysis.TextEdit{
			Pos: tokFile.Pos(source.Start(c.src, start)),
			End: tokFile.Pos(source.End(c.src, end)),
		}
	}
	return analysis.TextEdit{Pos: assign.Pos(), End: assign.End()}
}

// access maps each object to the positions where it is read and written.
type access map[types.Object]*live

// accesses returns the positions where each object is read and written. A read
// in a blank assignment that the rule can delete is left out: another such
// assignment cannot be a second use.
func accesses(pass *analysis.Pass, labeled map[*ast.AssignStmt]struct{}) access {
	var (
		out     = make(access)
		written = make(map[*ast.Ident]struct{})
		blank   = make(map[token.Pos]struct{})
	)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				out.assignment(pass, written, blank, labeled, file.Comments, n)
			case *ast.RangeStmt:
				out.rangeStmt(pass, written, n)
			case *ast.IncDecStmt:
				out.incDec(pass, n)
			case *ast.ValueSpec:
				out.valueSpec(pass, n)
			}
			return true
		})
	}
	for id, obj := range pass.TypesInfo.Uses {
		if _, ok := written[id]; ok {
			continue
		}
		if _, ok := blank[id.Pos()]; ok {
			continue
		}
		out.read(obj, id.Pos())
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					out.nakedReturns(pass, n.Type, n.Body)
				}
				return false
			case *ast.FuncLit:
				out.nakedReturns(pass, n.Type, n.Body)
				return false
			}
			return true
		})
	}
	return out
}

// nakedReturns records a read of each named result at every return in body
// that has no result expressions, and descends into function literals.
func (a access) nakedReturns(pass *analysis.Pass, typ *ast.FuncType, body *ast.BlockStmt) {
	results := namedResults(pass, typ)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			a.nakedReturns(pass, n.Type, n.Body)
			return false
		case *ast.ReturnStmt:
			if len(results) == 0 || len(n.Results) != 0 {
				return true
			}
			for _, result := range results {
				a.read(result, n.Pos())
			}
		}
		return true
	})
}

func namedResults(pass *analysis.Pass, typ *ast.FuncType) (out []*types.Var) {
	if typ.Results == nil {
		return nil
	}
	for _, field := range typ.Results.List {
		for _, id := range field.Names {
			if v, ok := pass.TypesInfo.Defs[id].(*types.Var); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

func (a access) entry(obj types.Object) *live {
	if a[obj] == nil {
		a[obj] = new(live)
	}
	return a[obj]
}

func (a access) write(obj types.Object, pos token.Pos) {
	if obj == nil {
		return
	}
	entry := a.entry(obj)
	entry.writes = append(entry.writes, pos)
}

func (a access) read(obj types.Object, pos token.Pos) {
	if obj == nil {
		return
	}
	entry := a.entry(obj)
	entry.reads = append(entry.reads, pos)
}

// assignment records the left-hand side identifiers of assign so that they do
// not count as reads: the target of = or := assigns its object rather than
// reading it. It also marks the reads in a blank assignment the rule can
// delete.
func (a access) assignment(pass *analysis.Pass, written map[*ast.Ident]struct{}, blank map[token.Pos]struct{}, labeled map[*ast.AssignStmt]struct{}, comments []*ast.CommentGroup, assign *ast.AssignStmt) {
	for _, lhs := range assign.Lhs {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if assign.Tok == token.ASSIGN || assign.Tok == token.DEFINE {
			written[id] = struct{}{}
		}
		a.write(obj, assign.End())
	}
	if !fixable(pass, assign, labeled, comments) {
		return
	}
	ast.Inspect(assign.Rhs[0], func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			blank[id.Pos()] = struct{}{}
		}
		return true
	})
}

func blankOf(assign *ast.AssignStmt) bool {
	if assign.Tok != token.ASSIGN {
		return false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}
	blank, ok := ast.Unparen(assign.Lhs[0]).(*ast.Ident)
	return ok && blank.Name == "_"
}

// fixable reports whether the fix can delete assign: it has a shape the rule
// reports, no label names it, and no comment would be lost.
func fixable(pass *analysis.Pass, assign *ast.AssignStmt, labeled map[*ast.AssignStmt]struct{}, comments []*ast.CommentGroup) bool {
	if !blankOf(assign) {
		return false
	}
	if _, ok := labeled[assign]; ok {
		return false
	}
	if commented(assign, comments, pass.Fset.File(assign.Pos())) {
		return false
	}
	return candidate(pass, ast.Unparen(assign.Rhs[0]))
}

// rangeStmt records the targets of r.
func (a access) rangeStmt(pass *analysis.Pass, written map[*ast.Ident]struct{}, r *ast.RangeStmt) {
	for _, target := range []ast.Expr{r.Key, r.Value} {
		id, ok := ast.Unparen(target).(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if r.Tok == token.ASSIGN {
			written[id] = struct{}{}
		}
		a.write(obj, r.Body.Lbrace)
	}
}

// incDec records the write that stmt makes.
func (a access) incDec(pass *analysis.Pass, stmt *ast.IncDecStmt) {
	id, ok := ast.Unparen(stmt.X).(*ast.Ident)
	if !ok {
		return
	}
	a.write(pass.TypesInfo.Uses[id], stmt.End())
}

// valueSpec records the writes that spec makes.
func (a access) valueSpec(pass *analysis.Pass, spec *ast.ValueSpec) {
	for _, id := range spec.Names {
		obj, ok := pass.TypesInfo.Defs[id].(*types.Var)
		if !ok {
			continue
		}
		a.write(obj, spec.End())
	}
}

// taken returns the objects whose explicit address is taken anywhere in the
// package.
func taken(pass *analysis.Pass) map[types.Object]struct{} {
	out := make(map[types.Object]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.UnaryExpr:
				if n.Op != token.AND {
					return true
				}
				mark(out, pass, n.X)
			case *ast.SelectorExpr:
				if !implicitAddress(pass, n) {
					return true
				}
				mark(out, pass, n.X)
			}
			return true
		})
	}
	return out
}

// mark records the variable whose storage expr denotes as address-taken.
func mark(out map[types.Object]struct{}, pass *analysis.Pass, expr ast.Expr) {
	id, ok := addressed(expr)
	if !ok {
		return
	}
	if obj, ok := pass.TypesInfo.Uses[id].(*types.Var); ok {
		out[obj] = struct{}{}
	}
}

// implicitAddress reports whether sel takes the address of its operand to call
// a method with a pointer receiver.
func implicitAddress(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	selection, ok := pass.TypesInfo.Selections[sel]
	if !ok {
		return false
	}
	fn, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return false
	}
	if _, ok := types.Unalias(recv.Type()).(*types.Pointer); !ok {
		return false
	}
	_, operand := types.Unalias(pass.TypesInfo.TypeOf(sel.X)).(*types.Pointer)
	return !operand
}

// captured returns the objects used inside a function literal. The literal may
// run at a call, so its source position does not order its uses with the code
// around it.
func captured(pass *analysis.Pass) map[types.Object]struct{} {
	out := make(map[types.Object]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.FuncLit)
			if !ok {
				return true
			}
			ast.Inspect(lit.Body, func(m ast.Node) bool {
				id, ok := m.(*ast.Ident)
				if !ok {
					return true
				}
				obj := pass.TypesInfo.Uses[id]
				if obj == nil {
					return true
				}
				if obj.Pos() < lit.Pos() || obj.Pos() > lit.End() {
					out[obj] = struct{}{}
				}
				return true
			})
			return true
		})
	}
	return out
}

// addressed returns the variable whose storage expr denotes: the identifier
// itself, or the variable at the root of a selector or index expression. A
// composite literal has no root variable to mark, and an index operand is read
// rather than addressed.
func addressed(expr ast.Expr) (*ast.Ident, bool) {
	switch expr := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return expr, true
	case *ast.SelectorExpr:
		return addressed(expr.X)
	case *ast.IndexExpr:
		return addressed(expr.X)
	case *ast.IndexListExpr:
		return addressed(expr.X)
	case *ast.StarExpr:
		return addressed(expr.X)
	}
	return nil, false
}

// labeled returns the labeled assignments. Deleting one would leave the label
// without a statement.
func labeled(pass *analysis.Pass) map[*ast.AssignStmt]struct{} {
	out := make(map[*ast.AssignStmt]struct{})
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			label, ok := n.(*ast.LabeledStmt)
			if !ok {
				return true
			}
			if assign, ok := label.Stmt.(*ast.AssignStmt); ok {
				out[assign] = struct{}{}
			}
			return true
		})
	}
	return out
}
