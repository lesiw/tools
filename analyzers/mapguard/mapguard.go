// Package mapguard provides an analysis.Analyzer that reports a presence check
// around a map write that needs none.
//
// The section of the specification on index expressions says a map index
// yields "the zero value for the element type of M" when the key is not
// present (https://go.dev/ref/spec#Index_expressions). Appending to that zero
// value returns the appended values alone, adding to it returns the summand,
// and incrementing it returns one, so each branch of the check writes the
// same thing.
//
// A += whose computed summand may be a float or a complex is left alone,
// because a summand of -0 adds to +0 where the branch that assigns stores -0.
//
// The fix replaces the if with the write in its then branch. An if that is
// another if's else becomes a block around that write. It is withheld where a
// comment inside the if, on the if's first or closing line, or between a
// branch and an else-if would be lost or left describing the check, where the
// write has no one-line form, or where the line it writes would run past the
// width limit while the guard's first line stays within it.
//
// Adapted from staticcheck's S1036, which reports the same guard.
package mapguard

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "mapguard",
	Doc:  "reports a presence check around a map write that needs none",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		parent := astexpr.Parents(file)
		ast.Inspect(file, func(n ast.Node) bool {
			if ifStmt, ok := n.(*ast.IfStmt); ok {
				examine(pass, file, parent[ifStmt], ifStmt)
			}
			return true
		})
	}
	return nil, nil
}

// examine reports ifStmt when each of its branches writes the same thing
// through the key it checks for.
func examine(pass *analysis.Pass, file *ast.File, owner ast.Node, ifStmt *ast.IfStmt) {
	index, flag, ok := presence(pass, ifStmt)
	// The fix reads the index a different number of times than the guard
	// did, so a key that can change between two reads is not the same key.
	if !ok || !astexpr.Stable(pass, index) {
		return
	}
	write, ok := branches(pass, ifStmt, index)
	// The write outlives the if, so it cannot read the flag the init
	// declared.
	if !ok || astexpr.Mentions(pass, write, flag) {
		return
	}
	d := analysis.Diagnostic{
		Pos:     ifStmt.Pos(),
		End:     ifStmt.End(),
		Message: "map write does not need a presence check",
	}
	outer := enclosing(owner, ifStmt)
	if edits, ok := fix(pass, file, outer, ifStmt, write); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the check",
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// enclosing returns the if that ifStmt is the else branch of, and nil for an
// if that stands on its own.
func enclosing(owner ast.Node, ifStmt *ast.IfStmt) *ast.IfStmt {
	outer, ok := owner.(*ast.IfStmt)
	if !ok || outer.Else != ifStmt {
		return nil
	}
	return outer
}

// presence returns the map index ifStmt's init reads and the flag it binds for
// the key's presence, and reports false for any other init or condition.
func presence(pass *analysis.Pass, ifStmt *ast.IfStmt) (*ast.IndexExpr, *ast.Ident, bool) {
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, nil, false
	}
	if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	blank, ok := ast.Unparen(assign.Lhs[0]).(*ast.Ident)
	if !ok || blank.Name != "_" {
		return nil, nil, false
	}
	flag, ok := ast.Unparen(assign.Lhs[1]).(*ast.Ident)
	if !ok || flag.Name == "_" {
		return nil, nil, false
	}
	index, ok := ast.Unparen(assign.Rhs[0]).(*ast.IndexExpr)
	if !ok || !astexpr.Same(pass, ifStmt.Cond, flag) {
		return nil, nil, false
	}
	return index, flag, true
}

// branches returns the statement ifStmt keeps when each of its branches writes
// the same thing through index.
func branches(pass *analysis.Pass, ifStmt *ast.IfStmt, index *ast.IndexExpr) (ast.Stmt, bool) {
	then, ok := only(ifStmt.Body)
	if !ok {
		return nil, false
	}
	other, ok := otherwise(ifStmt)
	if !ok {
		return nil, false
	}
	switch then := then.(type) {
	case *ast.IncDecStmt:
		if then.Tok != token.INC || !same(pass, then.X, index) {
			return nil, false
		}
		value, ok := assigned(pass, index, other)
		if ok && one(pass, value) {
			return then, true
		}
	case *ast.AssignStmt:
		switch then.Tok {
		case token.ADD_ASSIGN:
			if adds(pass, index, then, other) {
				return then, true
			}
		case token.ASSIGN:
			if appends(pass, index, then, other) {
				return then, true
			}
		}
	}
	return nil, false
}

// only returns the single statement body holds.
func only(body *ast.BlockStmt) (ast.Stmt, bool) {
	if len(body.List) != 1 {
		return nil, false
	}
	return body.List[0], true
}

// otherwise returns the single statement of ifStmt's else block.
func otherwise(ifStmt *ast.IfStmt) (ast.Stmt, bool) {
	block, ok := ifStmt.Else.(*ast.BlockStmt)
	if !ok {
		return nil, false
	}
	return only(block)
}

// assigned returns the value stmt assigns to index with =, and reports false
// for any other statement.
func assigned(pass *analysis.Pass, index *ast.IndexExpr, stmt ast.Stmt) (ast.Expr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	if !same(pass, assign.Lhs[0], index) {
		return nil, false
	}
	return assign.Rhs[0], true
}

// adds reports whether other assigns the value then adds to index.
func adds(pass *analysis.Pass, index *ast.IndexExpr, then *ast.AssignStmt, other ast.Stmt) bool {
	if len(then.Lhs) != 1 || len(then.Rhs) != 1 {
		return false
	}
	if !same(pass, then.Lhs[0], index) {
		return false
	}
	// A computed summand of -0 adds to +0, where the branch that assigns
	// stores -0. A constant is exact, so it is never -0.
	tv, ok := pass.TypesInfo.Types[then.Rhs[0]]
	if ok && tv.Value == nil && astexpr.MayBeNaN(tv.Type) {
		return false
	}
	value, ok := assigned(pass, index, other)
	return ok && sameText(value, then.Rhs[0])
}

// appends reports whether then assigns the result of appending to index, and
// other assigns the composite literal holding those values alone.
func appends(pass *analysis.Pass, index *ast.IndexExpr, then *ast.AssignStmt, other ast.Stmt) bool {
	value, ok := assigned(pass, index, then)
	if !ok {
		return false
	}
	call, ok := ast.Unparen(value).(*ast.CallExpr)
	if !ok || !builtinAppend(pass, call) {
		return false
	}
	// A spread appends the slice's elements, where the literal holds the
	// slice itself.
	if call.Ellipsis != token.NoPos {
		return false
	}
	// An append with no values returns the slice it was given, which is nil
	// for an absent key, where the literal is an empty slice.
	if len(call.Args) < 2 || !same(pass, call.Args[0], index) {
		return false
	}
	literal, ok := assigned(pass, index, other)
	if !ok {
		return false
	}
	values, ok := elements(literal)
	if !ok || len(values) != len(call.Args)-1 {
		return false
	}
	for i, value := range values {
		if !sameText(value, call.Args[i+1]) {
			return false
		}
	}
	return true
}

// builtinAppend reports whether call calls the builtin append.
func builtinAppend(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[ident] == types.Universe.Lookup("append")
}

// elements returns the elements of a composite literal, and reports false for
// an expression that is not one.
func elements(e ast.Expr) ([]ast.Expr, bool) {
	literal, ok := ast.Unparen(e).(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	return literal.Elts, true
}

// same reports whether a and b are the same expression: astexpr.Same covers
// the forms it names and compares a name by the object it resolves to, and
// astexpr.SamePure covers the rest, which needs the same printed text from a
// read that runs nothing.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	return astexpr.Same(pass, a, b) || astexpr.SamePure(pass, a, b)
}

// sameText reports whether a and b are written the same. The value in the else
// branch does not survive the fix, so a call in a value is no reason to
// refuse: exactly one of the two is evaluated before the fix, and exactly one
// after it.
func sameText(a, b ast.Expr) bool {
	return astexpr.Text(ast.Unparen(a)) == astexpr.Text(ast.Unparen(b))
}

// one reports whether e is the constant one, which is the value ++ adds.
func one(pass *analysis.Pass, e ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil {
		return false
	}
	n := constant.ToInt(tv.Value)
	return n.Kind() == constant.Int &&
		constant.Compare(n, token.EQL, constant.MakeInt64(1))
}

// fix returns the edit that writes stmt in place of the if, and reports false
// where the write has no one-line form, or where the edit would lose a comment
// or write a line past the width.
func fix(pass *analysis.Pass, file *ast.File, outer, ifStmt *ast.IfStmt, stmt ast.Stmt) ([]analysis.TextEdit, bool) {
	if commented(pass, file, outer, ifStmt) {
		return nil, false
	}
	body, ok := astexpr.OneLine(stmt)
	if !ok {
		return nil, false
	}
	src, ok := read(pass, ifStmt)
	if !ok {
		return nil, false
	}
	tokFile := pass.Fset.File(ifStmt.Pos())
	from := tokFile.Offset(ifStmt.Pos())
	to := tokFile.Offset(ifStmt.End())
	// The text was read after the file was parsed, and a file that shrank
	// in between would take the slices below past its end.
	if from > len(src) || to > len(src) || to < from {
		return nil, false
	}
	start := source.Start(src, from)
	stop := source.End(src, to)
	first := bytes.TrimRight(src[start:source.End(src, from)], "\r\n")
	prefix := src[start:from]
	replacement, line := body, slices.Concat(
		prefix,
		[]byte(body),
		bytes.TrimRight(src[to:stop], "\r\n"),
	)
	if outer != nil {
		indent := prefix[:len(prefix)-len(bytes.TrimLeft(prefix, " \t"))]
		replacement = "{\n" + string(indent) + "\t" + body + "\n" +
			string(indent) + "}"
		line = slices.Concat(indent, []byte("\t"+body))
	}
	if !fits(first, line) {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos:     ifStmt.Pos(),
		End:     ifStmt.End(),
		NewText: []byte(replacement),
	}}, true
}

// read returns the text of the file node is written in.
func read(pass *analysis.Pass, node ast.Node) ([]byte, bool) {
	file := pass.Fset.File(node.Pos())
	if file == nil {
		return nil, false
	}
	src, err := pass.ReadFile(file.Name())
	return src, err == nil
}

// commented reports whether a comment sits in the text the fix replaces, on
// the if's first or closing line, or between a branch and an else-if, where
// the edit would delete it or leave it describing the check.
func commented(pass *analysis.Pass, file *ast.File, outer, ifStmt *ast.IfStmt) bool {
	for _, group := range file.Comments {
		if group.Pos() < ifStmt.End() {
			if group.End() > ifStmt.Pos() {
				return true
			}
			if sameLine(pass, group.End(), ifStmt.Pos()) {
				return true
			}
			if outer != nil && group.End() > outer.Body.Rbrace {
				return true
			}
			continue
		}
		if sameLine(pass, group.Pos(), ifStmt.End()) {
			return true
		}
	}
	return false
}

// sameLine reports whether a and b are written on the same physical line.
func sameLine(pass *analysis.Pass, a, b token.Pos) bool {
	return pass.Fset.PositionFor(a, false).Line ==
		pass.Fset.PositionFor(b, false).Line
}

// fits reports whether line, the text the fix writes on the line the if
// starts, stays within the length limit. A first line already past the limit
// fits: refusing the fix cannot shorten it, and linelen reports the line
// either way. With no limit set, every line fits.
func fits(first, line []byte) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	return !linelen.CheckLine(first) || linelen.CheckLine(line)
}
