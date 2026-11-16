// Package rangestring provides an analysis.Analyzer that reports a range over
// a string that only counts runes.
//
// The spec says that a range over a string "iterates over the Unicode code
// points in the string" (https://go.dev/ref/spec#For_statements). The
// unicode/utf8 package documentation
// (https://pkg.go.dev/unicode/utf8#RuneCountInString) says that
// RuneCountInString "is like RuneCount but its input is a string", and
// RuneCount returns the number of runes, treating an erroneous encoding as one
// rune as the range does. A loop that starts a counter at zero and increments
// it for each code point computes the number of runes.
//
// The counter must be declared as n := 0 immediately before the loop, both
// iteration variables must be blank or absent, the loop must hold only n++,
// and the range expression must not refer to the counter. The fix writes a
// call to RuneCountInString in place of the declaration and the loop, adding
// the import where the file lacks one and converting a defined string type or
// a type parameter. The fix is withheld where the counter is not read outside
// the loop, where a comment sits in the text it replaces, where a name it
// writes reaches another object, and where the file binds unicode/utf8 to the
// dot or blank name.
//
// The README catalogs every check, exemption, and fix.
package rangestring

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "rangestring",
	Doc:  "reports a range over a string that only counts runes",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var (
			names = imports.Names(pass, file)
			stack []ast.Node
		)
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if loop, ok := n.(*ast.RangeStmt); ok {
				if decl, count, ok := counted(pass, loop, stack); ok {
					pass.Report(diagnose(pass, file, names, loop, decl, count))
				}
			}
			stack = append(stack, n)
			return true
		})
	}
	return nil, nil
}

// counted returns the declaration and increment that make loop a rune count,
// and reports whether loop has that shape. The declaration is the statement
// that names the counter and sets it to zero immediately before the loop.
func counted(pass *analysis.Pass, loop *ast.RangeStmt, stack []ast.Node) (*ast.AssignStmt, *ast.IncDecStmt, bool) {
	if !blank(loop.Key) || !blank(loop.Value) {
		return nil, nil, false
	}
	if !stringType(pass.TypesInfo.TypeOf(loop.X)) {
		return nil, nil, false
	}
	count, id, ok := increment(loop)
	if !ok {
		return nil, nil, false
	}
	decl, lhs, ok := zeroDecl(loop, stack)
	if !ok || pass.TypesInfo.ObjectOf(id) != pass.TypesInfo.ObjectOf(lhs) {
		return nil, nil, false
	}
	if astexpr.Mentions(pass, loop.X, id) {
		return nil, nil, false
	}
	return decl, count, true
}

// increment returns the counter and increment when loop's body holds only an
// increment of one identifier.
func increment(loop *ast.RangeStmt) (*ast.IncDecStmt, *ast.Ident, bool) {
	if len(loop.Body.List) != 1 {
		return nil, nil, false
	}
	count, ok := loop.Body.List[0].(*ast.IncDecStmt)
	if !ok || count.Tok != token.INC {
		return nil, nil, false
	}
	id, ok := ast.Unparen(count.X).(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	return count, id, true
}

// zeroDecl returns the declaration that sets the counter to zero immediately
// before loop, and the identifier it declares.
func zeroDecl(loop *ast.RangeStmt, stack []ast.Node) (*ast.AssignStmt, *ast.Ident, bool) {
	decl, ok := previous(loop, stack).(*ast.AssignStmt)
	if !ok || decl.Tok != token.DEFINE {
		return nil, nil, false
	}
	if len(decl.Lhs) != 1 || len(decl.Rhs) != 1 {
		return nil, nil, false
	}
	lhs, ok := decl.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	zero, ok := decl.Rhs[0].(*ast.BasicLit)
	if !ok || zero.Kind != token.INT || zero.Value != "0" {
		return nil, nil, false
	}
	return decl, lhs, true
}

// diagnose returns the diagnostic for one count loop, with the rewrite as a
// fix when it can be written.
func diagnose(pass *analysis.Pass, file *ast.File, names map[string]string, loop *ast.RangeStmt, decl *ast.AssignStmt, count *ast.IncDecStmt) analysis.Diagnostic {
	diag := analysis.Diagnostic{
		Pos:     loop.Pos(),
		End:     loop.End(),
		Message: "range counts runes",
	}
	if edits, ok := fix(pass, file, names, loop, decl, count); ok {
		if name, _, ok := qualifier(pass, names, decl.Pos()); ok {
			diag.Message += "; use " + name + ".RuneCountInString"
		}
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Count the runes with RuneCountInString",
			TextEdits: edits,
		}}
	}
	return diag
}

// fix returns the edits that replace the declaration and loop with a call to
// utf8.RuneCountInString, and reports whether the rewrite can be written.
func fix(pass *analysis.Pass, file *ast.File, names map[string]string, loop *ast.RangeStmt, decl *ast.AssignStmt, count *ast.IncDecStmt) ([]analysis.TextEdit, bool) {
	name, imported, ok := qualifier(pass, names, decl.Pos())
	if !ok {
		return nil, false
	}
	src := loop.X
	// RuneCountInString takes a string, so a defined string type or a type
	// parameter needs a conversion.
	wrapped := !isString(pass.TypesInfo.TypeOf(src))
	if wrapped && !predeclared(pass, "string", decl.Pos()) {
		return nil, false
	}
	id, ok := ast.Unparen(count.X).(*ast.Ident)
	if !ok {
		return nil, false
	}
	if !read(pass, file, pass.TypesInfo.ObjectOf(id), loop) {
		// The new declaration would have no use.
		return nil, false
	}
	head := id.Name + " := " + name + ".RuneCountInString("
	tail := ")"
	if wrapped {
		head += "string("
		tail = "))"
	}
	edits := []analysis.TextEdit{{
		Pos:     decl.Pos(),
		End:     src.Pos(),
		NewText: []byte(head),
	}, {
		Pos:     src.End(),
		End:     loop.End(),
		NewText: []byte(tail),
	}}
	for _, edit := range edits {
		if commented(file, edit.Pos, edit.End) {
			return nil, false
		}
	}
	if !imported {
		edits = append(edits, imports.Add(pass, file, "unicode/utf8"))
	}
	return edits, true
}

// qualifier returns the name the rewrite writes for unicode/utf8, whether the
// file imports it, and whether the name reaches that package at pos.
func qualifier(pass *analysis.Pass, names map[string]string, pos token.Pos) (name string, imported, ok bool) {
	name, ok = imports.Qualifier(pass, names, "unicode/utf8", pos)
	if !ok {
		return "", false, false
	}
	_, imported = names["unicode/utf8"]
	return name, imported, true
}

// blank reports whether e is the blank identifier, or absent.
func blank(e ast.Expr) bool {
	if e == nil {
		return true
	}
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// stringType reports whether a value of type t is a string, including a
// defined string type and a type parameter whose type set holds only strings.
func stringType(t types.Type) bool {
	return t != nil && types.AssignableTo(types.Typ[types.UntypedString], t)
}

// isString reports whether t is assignable to the predeclared string type
// without a conversion.
func isString(t types.Type) bool {
	return types.AssignableTo(t, types.Typ[types.String])
}

// predeclared reports whether name reaches the predeclared object at pos.
func predeclared(pass *analysis.Pass, name string, pos token.Pos) bool {
	return imports.Lookup(pass, name, pos) == types.Universe.Lookup(name)
}

// read reports whether obj is read somewhere in file outside loop. An
// identifier that only appears as the target of an assignment does not keep a
// variable in use.
func read(pass *analysis.Pass, file *ast.File, obj types.Object, loop *ast.RangeStmt) bool {
	var (
		found bool
		stack []ast.Node
	)
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if n == loop {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && !found {
			if pass.TypesInfo.Uses[id] == obj && !target(stack, id) {
				found = true
			}
		}
		stack = append(stack, n)
		return true
	})
	return found
}

// target reports whether id is an assignment target rather than a value read.
func target(stack []ast.Node, id *ast.Ident) bool {
	i := len(stack) - 1
	for i >= 0 {
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			break
		}
		i--
	}
	if i < 0 {
		return false
	}
	switch n := stack[i].(type) {
	case *ast.AssignStmt:
		if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
			return false
		}
		return holds(n.Lhs, id)
	case *ast.RangeStmt:
		return n.Tok != token.ILLEGAL && holds([]ast.Expr{n.Key, n.Value}, id)
	}
	return false
}

// holds reports whether exprs contains id, which may be parenthesized.
func holds(exprs []ast.Expr, id *ast.Ident) bool {
	for _, e := range exprs {
		if name, ok := ast.Unparen(e).(*ast.Ident); ok && name == id {
			return true
		}
	}
	return false
}

// previous returns the statement before loop in the block that holds it, or
// nil when no statement precedes loop in that list.
func previous(loop *ast.RangeStmt, stack []ast.Node) ast.Stmt {
	if len(stack) == 0 {
		return nil
	}
	var list []ast.Stmt
	switch n := stack[len(stack)-1].(type) {
	case *ast.BlockStmt:
		list = n.List
	case *ast.CaseClause:
		list = n.Body
	case *ast.CommClause:
		list = n.Body
	default:
		return nil
	}
	for i, stmt := range list {
		if stmt == loop && i > 0 {
			return list[i-1]
		}
	}
	return nil
}

// commented reports whether a comment sits in the span an edit would replace.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
