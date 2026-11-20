package unusedvariable

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

// syntax holds what an edit needs to know about a package: the parent and file
// of every node, the identifier at each position, and the identifier at each
// mention of an object, in the order the files hold them.
type syntax struct {
	pass   *analysis.Pass
	fset   *token.FileSet
	info   *types.Info
	place  map[ast.Node]place
	idents map[token.Pos]*ast.Ident
	uses   map[types.Object][]*ast.Ident
}

// place is where a node sits in the tree.
type place struct {
	parent ast.Node
	file   *ast.File
}

func newSyntax(pass *analysis.Pass) *syntax {
	src := &syntax{
		pass:   pass,
		fset:   pass.Fset,
		info:   pass.TypesInfo,
		place:  make(map[ast.Node]place),
		idents: make(map[token.Pos]*ast.Ident),
		uses:   make(map[types.Object][]*ast.Ident),
	}
	for _, file := range pass.Files {
		src.walk(file)
	}
	return src
}

// walk records the parent and file of every node in file, the identifier at
// each position, and the mentions of every object.
func (s *syntax) walk(file *ast.File) {
	stack := make([]ast.Node, 0, 8)
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			s.place[n] = place{parent: stack[len(stack)-1], file: file}
		}
		if id, ok := n.(*ast.Ident); ok {
			s.idents[id.Pos()] = id
			if obj := s.info.Uses[id]; obj != nil {
				s.uses[obj] = append(s.uses[obj], id)
			}
		}
		stack = append(stack, n)
		return true
	})
}

// parent returns the node that holds n, and nil for a file.
func (s *syntax) parent(n ast.Node) ast.Node { return s.place[n].parent }

// file returns the file n belongs to.
func (s *syntax) file(n ast.Node) *ast.File { return s.place[n].file }

// line returns the line of pos, ignoring line directives.
func (s *syntax) line(pos token.Pos) int {
	return s.fset.PositionFor(pos, false).Line
}

// survives reports whether the package names the object id refers to anywhere
// but the declaration. A redeclaring short declaration puts the name in Uses,
// so the object has to be looked up rather than read from Defs.
func (s *syntax) survives(id *ast.Ident) bool {
	obj := s.info.ObjectOf(id)
	return obj != nil && len(s.uses[obj]) > 0
}

// deleteVar returns the edits that remove the variable whose defining
// identifier is id, and nil when id names nothing this rule can remove, as a
// range clause does.
func (s *syntax) deleteVar(id *ast.Ident) []analysis.TextEdit {
	switch parent := s.parent(id).(type) {
	case *ast.ValueSpec:
		if slices.Contains(parent.Names, id) {
			return s.deleteFromSpec(parent, id)
		}
	case *ast.AssignStmt:
		index := slices.Index(parent.Lhs, ast.Expr(id))
		if index < 0 {
			return nil
		}
		if s.clause(parent) {
			return s.deleteFromClause(parent, id)
		}
		return s.deleteFromAssign(parent, id, index)
	}
	return nil
}

// writes returns the edits that remove every mention of the variable id
// declares. Every mention is a write: a read would keep the compiler from
// reporting the variable at all.
func (s *syntax) writes(id *ast.Ident) (edits []analysis.TextEdit) {
	for _, use := range s.uses[s.info.Defs[id]] {
		// The write is to the whole target, parentheses and all.
		target := ast.Expr(use)
		for {
			paren, ok := s.parent(target).(*ast.ParenExpr)
			if !ok {
				break
			}
			target = paren
		}
		// A write that carries nothing else goes whole when it stands alone
		// on its line.
		assign, ok := s.parent(target).(*ast.AssignStmt)
		alone := ok && !another(assign.Lhs, target)
		if alone && !hasEffects(s.pass, assign.Rhs) {
			if cut := s.deleteStmt(assign); cut != nil {
				edits = append(edits, cut...)
				continue
			}
		}
		edits = append(edits, blank(target))
	}
	return edits
}

// clause reports whether assign is the guard of a type switch or the
// communication of a select clause, where the name is a prefix of an
// expression rather than a declaration of its own.
func (s *syntax) clause(assign *ast.AssignStmt) bool {
	switch parent := s.parent(assign).(type) {
	case *ast.TypeSwitchStmt:
		return parent.Assign == assign
	case *ast.CommClause:
		return parent.Comm == assign
	}
	return false
}

// deleteFromClause returns the edits that remove a name from a guard or a
// receive clause. The expression stays, so the declaration of the name goes
// with it, unless another name of the clause reads a value the clause takes.
func (s *syntax) deleteFromClause(assign *ast.AssignStmt, id *ast.Ident) []analysis.TextEdit {
	if s.doomedLhs(assign.Lhs, id) {
		return []analysis.TextEdit{{
			Pos: assign.Pos(),
			End: assign.Rhs[0].Pos(),
		}}
	}
	return []analysis.TextEdit{blank(id)}
}

// deleteFromSpec returns the edits that remove one name from a var spec,
// keeping the spec when it still declares a name or evaluates a value.
func (s *syntax) deleteFromSpec(spec *ast.ValueSpec, id *ast.Ident) []analysis.TextEdit {
	if s.doomedSpec(spec, id) {
		if edits := s.deleteSpec(spec, id); len(edits) > 0 {
			return edits
		}
		// The declaration cannot go whole, as one a label holds cannot, so
		// the name becomes a blank.
		return []analysis.TextEdit{blank(id)}
	}
	index := slices.Index(spec.Names, id)

	// The spec still declares a name the package reads, so only this one
	// goes, with the comma that separates it from a name that stays.
	if len(spec.Values) == 0 && len(spec.Names) > 1 {
		return []analysis.TextEdit{cut(spec.Names, index)}
	}

	// A value with no effect goes with the name it was assigned to, which is
	// only known to pair up when the two sides are the same length.
	paired := len(spec.Names) == len(spec.Values) && len(spec.Names) > 1
	if paired && noEffects(s.pass, spec.Values[index]) {
		return []analysis.TextEdit{
			cut(spec.Names, index),
			cut(spec.Values, index),
		}
	}
	return []analysis.TextEdit{blank(id)}
}

// deleteFromAssign returns the edits that remove one name from the left of an
// assignment, keeping the right side when evaluating it does something.
func (s *syntax) deleteFromAssign(assign *ast.AssignStmt, id *ast.Ident, index int) (edits []analysis.TextEdit) {
	if s.doomedLhs(assign.Lhs, id) {
		return s.clearAssign(assign)
	}

	// Another name of the statement is read elsewhere, so the statement keeps
	// its shape. A value with no effect goes with the name it was assigned to,
	// which is only known to pair up when the two sides are the same length.
	paired := len(assign.Lhs) == len(assign.Rhs) && len(assign.Lhs) > 1
	if paired && noEffects(s.pass, assign.Rhs[index]) {
		edits = append(edits,
			cut(assign.Lhs, index),
			cut(assign.Rhs, index),
		)
	} else {
		edits = append(edits, blank(id))
	}
	// A short declaration that no longer declares a name is a compile error,
	// so the token becomes a plain assignment.
	if assign.Tok == token.DEFINE && !s.declaresNew(assign.Lhs, index) {
		edits = append(edits, define(assign))
	}
	return edits
}

// clearAssign returns the edits that remove an assignment with an effect-free
// right side, and that blank its names when a value has to be evaluated.
func (s *syntax) clearAssign(assign *ast.AssignStmt) (edits []analysis.TextEdit) {
	if !hasEffects(s.pass, assign.Rhs) {
		// The statement is only a declaration, so it goes whole when it
		// stands alone.
		if cut := s.deleteStmt(assign); cut != nil {
			return cut
		}
	}
	// The values have to be evaluated, so every name becomes a blank. None is
	// left declared, so a short declaration becomes an assignment.
	for _, expr := range assign.Lhs {
		if name, ok := expr.(*ast.Ident); ok {
			edits = append(edits, blank(name))
		}
	}
	if assign.Tok == token.DEFINE {
		edits = append(edits, define(assign))
	}
	return edits
}

// define returns the edit that turns a short declaration into an assignment.
func define(assign *ast.AssignStmt) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     assign.TokPos,
		End:     assign.TokPos + token.Pos(len(":=")),
		NewText: []byte("="),
	}
}

// declaresNew reports whether the left side declares a name other than the one
// at index, which is what a short declaration needs to keep its token.
func (s *syntax) declaresNew(lhs []ast.Expr, index int) bool {
	for i, expr := range lhs {
		if i == index {
			continue
		}
		if name, ok := expr.(*ast.Ident); ok && name.Name != "_" {
			if s.info.Defs[name] != nil {
				return true
			}
		}
	}
	return false
}

// doomed reports whether names declare no variable that the package names
// anywhere else. The variable id declares is left out: the fix removes its
// mentions too.
func (s *syntax) doomed(names []*ast.Ident, id *ast.Ident) bool {
	return !slices.ContainsFunc(names, func(name *ast.Ident) bool {
		return name != id && name.Name != "_" && s.survives(name)
	})
}

// doomedLhs reports whether the left side of an assignment declares no
// variable that the package names anywhere else.
func (s *syntax) doomedLhs(exprs []ast.Expr, id *ast.Ident) bool {
	var names []*ast.Ident
	for _, expr := range exprs {
		if name, ok := expr.(*ast.Ident); ok {
			names = append(names, name)
		}
	}
	return s.doomed(names, id)
}

// doomedSpec reports whether the spec declares no variable that the package
// names anywhere else and evaluates nothing, so that removing id removes all
// of it.
func (s *syntax) doomedSpec(spec *ast.ValueSpec, id *ast.Ident) bool {
	return s.doomed(spec.Names, id) && !hasEffects(s.pass, spec.Values)
}

// doomedDecl reports whether the declaration holds only specs this rule
// removes whole, so that removing one removes them all.
func (s *syntax) doomedDecl(decl *ast.GenDecl, id *ast.Ident) bool {
	return !slices.ContainsFunc(decl.Specs, func(spec ast.Spec) bool {
		return !s.doomedSpec(spec.(*ast.ValueSpec), id)
	})
}

// deleteSpec returns the edits that remove a var spec and its doc comment,
// taking the whole declaration with it when every spec goes.
func (s *syntax) deleteSpec(spec *ast.ValueSpec, id *ast.Ident) []analysis.TextEdit {
	var (
		decl  = s.parent(spec).(*ast.GenDecl)
		index = slices.Index(decl.Specs, ast.Spec(spec))
	)
	if s.doomedDecl(decl, id) {
		return s.deleteDecl(decl)
	}
	pos, end := spec.Pos(), spec.End()
	if spec.Doc != nil {
		pos = spec.Doc.Pos()
	}
	switch {
	case index < len(decl.Specs)-1:
		next := decl.Specs[index+1]
		end = next.Pos()
		if value, ok := next.(*ast.ValueSpec); ok && value.Doc != nil {
			end = value.Doc.Pos()
		}
	default:
		if spec.Comment != nil {
			end = spec.Comment.End()
		}
		// Nothing follows the spec on its line, so the line goes too.
		if s.line(decl.Rparen) > s.line(end) {
			end = s.eol(end)
		}
	}
	return []analysis.TextEdit{{Pos: pos, End: end}}
}

// deleteDecl returns the edits that remove a declaration statement and its doc
// comment, and nil for one a label holds, since removing it would leave the
// label without a statement.
func (s *syntax) deleteDecl(decl *ast.GenDecl) []analysis.TextEdit {
	stmt, ok := s.parent(decl).(*ast.DeclStmt)
	if !ok {
		return nil
	}
	return s.deleteStmt(stmt)
}

// another reports whether exprs holds a target other than target: a name the
// compiler tracks, or anything that is not a name at all, as a field or an
// element the assignment updates.
func another(exprs []ast.Expr, target ast.Expr) bool {
	return slices.ContainsFunc(exprs, func(expr ast.Expr) bool {
		if expr == target {
			return false
		}
		name, ok := expr.(*ast.Ident)
		return !ok || name.Name != "_"
	})
}

// hasEffects reports whether evaluating any of exprs does something.
func hasEffects(pass *analysis.Pass, exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if !noEffects(pass, expr) {
			return true
		}
	}
	return false
}

// cut returns the edit that removes one item of a list of two or more, with
// the separator that follows it or precedes it. A list of one has no separator
// to remove, and its callers remove the item whole.
func cut[T ast.Node](items []T, index int) analysis.TextEdit {
	if index == len(items)-1 {
		return analysis.TextEdit{
			Pos: items[index-1].End(),
			End: items[index].End(),
		}
	}
	return analysis.TextEdit{
		Pos: items[index].Pos(),
		End: items[index+1].Pos(),
	}
}

// blank returns the edit that writes the blank identifier over target.
func blank(target ast.Expr) analysis.TextEdit {
	return analysis.TextEdit{
		Pos:     target.Pos(),
		End:     target.End(),
		NewText: []byte("_"),
	}
}

// noEffects reports whether evaluating expr only computes a value: it runs no
// code and takes nothing from a channel. A panic it could raise is not
// counted, because the package does not build until the variable is gone, so
// the expression has not run; astexpr.Safe is the stricter question, and
// refuses an expression that can panic.
func noEffects(pass *analysis.Pass, expr ast.Expr) bool {
	clean := true
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n := n.(type) {
		case nil, *ast.Ident, *ast.BasicLit, *ast.BinaryExpr, *ast.ParenExpr,
			*ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr,
			*ast.TypeAssertExpr, *ast.StarExpr, *ast.CompositeLit,
			// not expressions, but they sit inside one
			*ast.KeyValueExpr, *ast.FieldList, *ast.Field, *ast.Ellipsis,
			*ast.IndexListExpr:
		case *ast.ArrayType, *ast.StructType, *ast.ChanType, *ast.FuncType,
			*ast.MapType, *ast.InterfaceType:
			return false // a type, and everything in it
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				clean = false // a receive takes an element
			}
		case *ast.CallExpr:
			if !astexpr.Pure(pass, n) && !allocates(pass, n) {
				clean = false
			}
		case *ast.FuncLit:
			return false // creating a function runs nothing
		default:
			clean = false
		}
		return clean
	})
	return clean
}

// allocates reports whether a call is make or new. astexpr.Pure leaves those
// out, because a second evaluation allocates a second value; here the question
// is only whether dropping the call drops what somebody reads, and the result
// of an allocation nobody keeps is nothing.
func allocates(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.ObjectOf(ident).(*types.Builtin)
	return ok && (builtin.Name() == "make" || builtin.Name() == "new")
}
