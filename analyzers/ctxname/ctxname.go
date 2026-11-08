// Package ctxname provides an analysis.Analyzer that reports
// context.Context variables that can be named ctx.
//
// A function-scoped variable, parameter, or named result of type
// context.Context should be named ctx, and it is reported only
// when the rename provably cannot rebind anything: no identifier
// spelled ctx — including a selector's field name — appears in
// the enclosing function after the candidate's declaration or
// lives before it without a legal redeclaration, and no sibling
// candidate shares the function. A second context alongside a
// live ctx is the one legitimate case for another name, so it is
// not reported at all. The parent parameter of a function that
// derives and returns a context — the standard library's own
// WithCancel(parent Context) shape — keeps its name. References
// inside the candidate's own declaration statement stay on the
// outer binding — ctx, cancel := context.WithTimeout(ctx, d) is
// the canonical shadow — so they never block the rename.
//
// A struct field of type context.Context should also be named ctx,
// as the standard library names its own; a field is reported
// unless a sibling field already claims the name.
//
// Package-scope variables and parameter names in interface methods
// and function-type declarations are not reported — an interface
// method need not name its parameters at all — and a variable
// whose declared type is an alias of context.Context is not
// matched. Renaming is editor work,
// so no fix is offered.
package ctxname

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ctxname",
	Doc:  "reports context.Context variables that can be named ctx",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		checkFile(pass, f)
	}
	return nil, nil
}

type candidate struct {
	def  *ast.Ident
	fn   ast.Node // enclosing function declaration or literal
	stmt ast.Stmt // statement declaring the candidate, if any
}

func checkFile(pass *analysis.Pass, f *ast.File) {
	var (
		stack      []ast.Node
		cands      []*candidate
		ctxNamed   []*ast.Ident
		fieldDiags []analysis.Diagnostic

		info = pass.TypesInfo
	)
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if id.Name == "ctx" {
			ctxNamed = append(ctxNamed, id)
		}
		obj, ok := info.Defs[id].(*types.Var)
		if !ok || !wantsCtx(id, obj) {
			return true
		}
		if obj.Parent() == nil {
			fieldDiags = checkField(id, stack, fieldDiags)
			return true
		}
		c := &candidate{
			def:  id,
			fn:   enclosingFunc(stack),
			stmt: enclosingStmt(stack),
		}
		if c.fn == nil {
			return true
		}
		parental := id.Name == "parent" && c.stmt == nil
		if parental && derivesCtx(info, stack) {
			return true
		}
		cands = append(cands, c)
		return true
	})
	shared := make(map[ast.Node]int)
	for _, c := range cands {
		shared[c.fn]++
	}
	for _, c := range cands {
		if shared[c.fn] > 1 {
			continue
		}
		if captured(info, c, ctxNamed) {
			continue
		}
		report(pass, c)
	}
	for _, d := range fieldDiags {
		pass.Report(d)
	}
}

// checkField collects a diagnostic when id declares a struct field
// of type context.Context under another name. Selector uses cannot
// rebind, so the only stand-down is a sibling field already named
// ctx.
func checkField(id *ast.Ident, stack []ast.Node, diags []analysis.Diagnostic) []analysis.Diagnostic {
	if len(stack) < 4 {
		return diags
	}
	st, ok := stack[len(stack)-4].(*ast.StructType)
	if !ok {
		return diags
	}
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			if name.Name == "ctx" {
				return diags
			}
		}
	}
	return append(diags, analysis.Diagnostic{
		Pos: id.Pos(),
		End: id.End(),
		Message: "context.Context should be named ctx, not " +
			id.Name,
	})
}

// wantsCtx reports whether obj is a context.Context under the wrong
// name. Blank identifiers have a nil parent alongside struct
// fields, which checkField owns; package-scope candidates fall out
// later, having no enclosing function.
func wantsCtx(id *ast.Ident, obj *types.Var) bool {
	if id.Name == "ctx" || id.Name == "_" {
		return false
	}
	return obj.Type().String() == "context.Context"
}

// captured reports whether renaming c to ctx could rebind a name:
// some ident spelled ctx sits in c's function outside c's own
// declaration statement. A ctx referenced below the declaration
// would be shadowed by the rename, a ctx declared below would
// capture c's uses, and a ctx live above would collide with the
// redeclaration.
func captured(info *types.Info, c *candidate, ctxNamed []*ast.Ident) bool {
	for _, id := range ctxNamed {
		if id.Pos() < c.fn.Pos() || id.Pos() >= c.fn.End() {
			continue
		}
		if c.stmt != nil && id.Pos() < c.stmt.End() {
			if definedBeside(c.stmt, id) {
				// rctx, ctx := f(): the sibling name is already
				// claimed in the candidate's own declaration.
				return true
			}
			if id.Pos() >= c.stmt.Pos() || redeclares(info, c) {
				// ctx, cancel := context.WithTimeout(ctx, d):
				// the reference inside the declaration is the
				// intended shadow, and a mixed := redeclares an
				// earlier ctx legally.
				continue
			}
			return true
		}
		return true
	}
	return false
}

// definedBeside reports whether id is a name stmt itself defines:
// a ctx on the left of the candidate's own declaration already
// claims the name.
func definedBeside(stmt ast.Stmt, id *ast.Ident) bool {
	switch node := stmt.(type) {
	case *ast.AssignStmt:
		return slices.Contains(node.Lhs, ast.Expr(id))
	case *ast.DeclStmt:
		gd, ok := node.Decl.(*ast.GenDecl)
		if !ok {
			return false
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if ok && slices.Contains(vs.Names, id) {
				return true
			}
		}
	}
	return false
}

// redeclares reports whether the candidate's short declaration
// defines another new variable besides the candidate, so renaming
// the candidate to an existing name still compiles as a
// redeclaration. A sibling that is itself a redeclaration does not
// count: a := needs at least one new name, and the rename spends
// the candidate's.
func redeclares(info *types.Info, c *candidate) bool {
	assign, ok := c.stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return false
	}
	for _, lhs := range assign.Lhs {
		id, ok := lhs.(*ast.Ident)
		if ok && id != c.def && info.Defs[id] != nil {
			return true
		}
	}
	return false
}

// report carries no fix: renaming is editor work.
func report(pass *analysis.Pass, c *candidate) {
	pass.Report(analysis.Diagnostic{
		Pos: c.def.Pos(),
		End: c.def.End(),
		Message: "context.Context should be named ctx, not " +
			c.def.Name,
	})
}

// derivesCtx reports whether the innermost enclosing function
// returns a context.Context.
func derivesCtx(info *types.Info, stack []ast.Node) bool {
	for _, s := range slices.Backward(stack) {
		var ft *ast.FuncType
		switch node := s.(type) {
		case *ast.FuncDecl:
			ft = node.Type
		case *ast.FuncLit:
			ft = node.Type
		default:
			continue
		}
		if ft.Results == nil {
			return false
		}
		for _, field := range ft.Results.List {
			t := info.TypeOf(field.Type)
			if t != nil && t.String() == "context.Context" {
				return true
			}
		}
		return false
	}
	return false
}

// enclosingFunc returns the innermost function on the stack.
func enclosingFunc(stack []ast.Node) ast.Node {
	for _, s := range slices.Backward(stack) {
		switch s.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return s
		}
	}
	return nil
}

// enclosingStmt returns the statement declaring the innermost ident
// on the stack, or nil for a parameter: a field is reached before
// any statement, and a parameter has no declaration statement — the
// binding of an enclosing function literal must not become an
// excusal window for its own body.
func enclosingStmt(stack []ast.Node) ast.Stmt {
	for _, s := range slices.Backward(stack) {
		if _, ok := s.(*ast.Field); ok {
			return nil
		}
		if stmt, ok := s.(ast.Stmt); ok {
			return stmt
		}
	}
	return nil
}
