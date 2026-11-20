// Package unusedfunc provides an analysis.Analyzer that reports a declaration
// at package level that nothing in the package refers to.
//
// Vendored from gopls' unusedfunc
// (gopls/internal/analysis/unusedfunc, golang.org/x/tools/gopls v0.24.0),
// which is internal and cannot be imported. The rule is upstream's; the code
// is adapted to this repository's helpers. The fixes are written here, since
// upstream would delete a specification that the one below it repeats.
//
// A package's source cannot show every caller. Assembly calls a function by
// name, another package may reach one through a //go:linkname directive, a
// program may look a name up at run time, C calls one through cgo's //export,
// and a WASM host calls one through //go:wasmexport. A file the build excludes
// is not read: its callers are invisible, as upstream leaves them, so a
// declaration called only from such a file is reported.
//
// A //go:linkname directive on the declaration marks the name as another
// package's, so that declaration is left alone. The other callers are
// invisible, so those declarations are reported, and no fix deletes a symbol
// that //export or //go:wasmexport names. The Go toolchain is left alone,
// because those callers are common there.
//
// A fix removes the declaration and the comment that documents it, and nothing
// else: a specification that holds another name or a value with an effect
// keeps what it needs, and a name whose removal would change what a
// neighbouring specification declares is blanked. No edit is offered where a
// caller may be outside what this run can see, as assembly is. The README
// lists every check and fix.
package unusedfunc

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedfunc",
	Doc:  "reports a package-level declaration nothing in the package refers to",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if standard(pass) {
		return nil, nil
	}
	c := checker{
		pass:   pass,
		info:   pass.TypesInfo,
		ifaces: uses.InterfaceMethods(pass),
		refs:   newReferences(pass),
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			c.decl(file, decl)
		}
	}
	return nil, nil
}

// checker reports the declarations of one package that nothing refers to.
type checker struct {
	pass   *analysis.Pass
	info   *types.Info
	ifaces uses.Declarations // the unexported interface methods of the package
	refs   references
}

// decl checks the declarations of file that can be reported.
func (c *checker) decl(file *ast.File, decl ast.Decl) {
	if linknamed(decl) {
		return // another package names it in a //go:linkname line
	}
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		c.function(file, decl)
	case *ast.GenDecl:
		switch decl.Tok {
		case token.TYPE:
			c.types(file, decl)
		case token.CONST, token.VAR:
			c.values(file, decl)
		}
	}
}

// function checks a function or method declaration.
func (c *checker) function(file *ast.File, decl *ast.FuncDecl) {
	id := decl.Name
	if decl.Recv != nil {
		// The name is compared rather than the signature, as upstream does, so
		// a method whose signature differs from a same-named interface method
		// is left alone.
		if _, ok := c.ifaces[id.Name]; ok {
			return // it may be called through the interface
		}
	}
	if decl.Recv == nil && (id.Name == "init" || id.Name == "main") {
		return // the runtime calls these
	}
	noun := "function"
	if decl.Recv != nil {
		noun = "method"
	}
	var edits []analysis.TextEdit
	if !attached(decl) {
		edits = deleteDecl(c.pass.Fset.File(file.Pos()), file, decl)
	}
	c.check(noun, id, decl, edits)
}

// attached reports whether something outside the package names fn without a
// reference: C through cgo's //export, a WASM host through //go:wasmexport, or
// assembly, for which the declaration has no body at all.
func attached(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return true
	}
	doc := docOf(fn)
	if doc == nil {
		return false
	}
	for _, comment := range doc.List {
		if strings.HasPrefix(comment.Text, "//export ") {
			return true
		}
		if strings.HasPrefix(comment.Text, "//go:wasmexport ") {
			return true
		}
	}
	return false
}

// types checks each specification of a type declaration.
func (c *checker) types(file *ast.File, decl *ast.GenDecl) {
	var (
		tokFile = c.pass.Fset.File(file.Pos())
		all     = c.whole(decl)
	)
	for i, spec := range decl.Specs {
		spec := spec.(*ast.TypeSpec)
		switch {
		case !all:
			c.check(
				"type", spec.Name, spec,
				deleteSpec(tokFile, file, decl, i),
			)
		case i == 0:
			// Every specification goes, so one edit removes them all
			// rather than leaving an empty block behind.
			c.check("type", spec.Name, spec, deleteDecl(tokFile, file, decl))
		default:
			c.check("type", spec.Name, spec, nil)
		}
	}
}

// values checks each name a const or var declaration introduces.
func (c *checker) values(file *ast.File, decl *ast.GenDecl) {
	if enum(c.info, decl) {
		c.set(file, decl)
		return
	}
	var (
		tokFile = c.pass.Fset.File(file.Pos())
		all     = c.whole(decl)
	)
	for i, spec := range decl.Specs {
		spec := spec.(*ast.ValueSpec)
		if linknamed(spec) {
			continue // another package names it in a //go:linkname line
		}
		var fixed bool // whether a name here already carries an edit
		for j, id := range spec.Names {
			var edits []analysis.TextEdit
			switch {
			case !all && !fixed:
				edits = deleteVar(tokFile, c.pass, file, decl, i, j)
			case i == 0 && j == 0:
				edits = deleteDecl(tokFile, file, decl)
			}
			if c.check(decl.Tok.String(), id, spec, edits) {
				// One edit per specification per round: the edits for two
				// names in it would share text, and a driver that applies one
				// fix at a time would apply the second to text the first has
				// already changed. The names beside this one follow when the
				// tool runs again.
				fixed = true
			}
		}
	}
}

// whole reports whether the declaration can be removed in one edit: nothing it
// declares is used, nothing it evaluates has to stay, and no value names what
// the declaration declares.
func (c *checker) whole(decl *ast.GenDecl) bool {
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			if c.used(spec.Name, spec) {
				return false
			}
		case *ast.ValueSpec:
			if linknamed(spec) || effectful(c.pass, spec) {
				return false
			}
			for _, id := range spec.Names {
				if c.used(id, spec) {
					return false
				}
				if selfRead(c.info, spec, id) {
					return false
				}
			}
		}
	}
	return true
}

// set checks a group of constants that all share one type. A use of one name
// is a use of the group, since some values are written to keep the group
// symmetric or its ordering intact rather than to be named.
func (c *checker) set(file *ast.File, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		for _, id := range spec.(*ast.ValueSpec).Names {
			if c.used(id, spec) {
				return
			}
		}
	}
	var (
		pass  = c.pass
		edits = deleteDecl(pass.Fset.File(file.Pos()), file, decl)
		d     = analysis.Diagnostic{
			Pos:     decl.Pos(),
			End:     decl.End(),
			Message: "all values in this set of constants are unused",
		}
	)
	if edits != nil {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the constants declaration",
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// check reports id as unused, with edits as its fix, unless the package may
// refer to it, and reports whether the diagnostic carries a fix. A nil edits
// reports the diagnostic and offers none.
func (c *checker) check(noun string, id *ast.Ident, self ast.Node, edits []analysis.TextEdit) bool {
	if c.used(id, self) {
		return false
	}
	d := analysis.Diagnostic{
		Pos:     id.Pos(),
		End:     id.End(),
		Message: fmt.Sprintf("%s %q is unused", noun, id.Name),
	}
	if edits != nil {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   fmt.Sprintf("Remove %s %q", noun, id.Name),
			TextEdits: edits,
		}}
	}
	c.pass.Report(d)
	return edits != nil
}

// references holds the place of every reference the package makes to an
// object.
type references map[types.Object][]token.Pos

// newReferences returns the references the package makes, keyed by the object
// they name.
func newReferences(pass *analysis.Pass) references {
	out := make(references)
	for id, obj := range pass.TypesInfo.Uses {
		obj = origin(obj)
		out[obj] = append(out[obj], id.Pos())
	}
	return out
}

// origin returns the method obj was declared as when obj is a method seen
// through an instantiated type, so that a use of G[int].method counts as a use
// of G[T].method.
func origin(obj types.Object) types.Object {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Signature().Recv() == nil {
		return obj
	}
	return fn.Origin()
}

// used reports whether the package may refer to the object id declares from
// outside self. An exported name may be referred to from another package, and
// the blank identifier declares nothing, so both are used by construction. A
// reference inside self is the declaration naming itself, as a recursive
// function or a type that mentions its own name does.
func (c *checker) used(id *ast.Ident, self ast.Node) bool {
	if id.IsExported() || id.Name == "_" {
		return true
	}
	for _, pos := range c.refs[c.info.Defs[id]] {
		if pos < self.Pos() || pos > self.End() {
			return true
		}
	}
	return false
}

// enum reports whether decl is a group of constants that all share one type.
func enum(info *types.Info, decl *ast.GenDecl) bool {
	if decl.Tok != token.CONST || len(decl.Specs) < 2 {
		return false
	}
	var prev types.Type
	for _, spec := range decl.Specs {
		for _, id := range spec.(*ast.ValueSpec).Names {
			cur := info.TypeOf(id)
			if prev != nil && !types.Identical(cur, prev) {
				return false
			}
			prev = cur
		}
	}
	return true
}

// linknamed reports whether a //go:linkname directive precedes n, which names
// what n declares from another package.
func linknamed(n ast.Node) bool {
	doc := docOf(n)
	if doc == nil {
		return false
	}
	for _, comment := range doc.List {
		if strings.HasPrefix(comment.Text, "//go:linkname ") {
			return true
		}
	}
	return false
}

// standard reports whether the package under analysis is part of the Go
// toolchain, which it is when its files sit under GOROOT's src directory. The
// answer comes from the file names the pass already holds.
//
// The module a package belongs to does not answer the question. A driver
// reports no module for the standard library and none for a tree built without
// modules either, so the two cannot be told apart that way.
func standard(pass *analysis.Pass) bool {
	root := filepath.Join(build.Default.GOROOT, "src")
	root += string(filepath.Separator)
	for _, f := range pass.Files {
		if strings.HasPrefix(pass.Fset.Position(f.Pos()).Filename, root) {
			return true
		}
	}
	return false
}

// deleteDecl returns the edits that remove decl, its doc comment, and any
// comment on the line it ends on. When another declaration follows on that
// line, the edit stops at its start, so one separator between the two
// survives.
func deleteDecl(tokFile *token.File, file *ast.File, decl ast.Decl) []analysis.TextEdit {
	pos, end := decl.Pos(), decl.End()
	if doc := docOf(decl); doc != nil {
		pos = doc.Pos()
	}
	if next, ok := nextOnLine(tokFile, file, decl); ok {
		// The edit ends at the declaration that follows, which keeps the
		// separator after it.
		return []analysis.TextEdit{{Pos: pos, End: next}}
	}
	endLine := lineOf(tokFile, end)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			onEnd := lineOf(tokFile, comment.Pos()) == endLine &&
				lineOf(tokFile, comment.End()) == endLine
			if comment.Pos() >= end && onEnd {
				end = comment.End()
			}
		}
	}
	return []analysis.TextEdit{{Pos: pos, End: end}}
}

// nextOnLine returns the position of the declaration that follows decl on the
// line decl ends on.
func nextOnLine(tokFile *token.File, file *ast.File, decl ast.Decl) (token.Pos, bool) {
	endLine := lineOf(tokFile, decl.End())
	for _, other := range file.Decls {
		if other.Pos() <= decl.End() {
			continue
		}
		return other.Pos(), lineOf(tokFile, other.Pos()) == endLine
	}
	return token.NoPos, false
}

// deleteSpec returns the edits that remove the specification at index, or the
// whole declaration when it holds nothing else.
func deleteSpec(tokFile *token.File, file *ast.File, decl *ast.GenDecl, index int) []analysis.TextEdit {
	if len(decl.Specs) == 1 {
		return deleteDecl(tokFile, file, decl)
	}
	var (
		spec     = decl.Specs[index]
		pos, end = spec.Pos(), spec.End()
	)
	if doc := docOf(spec); doc != nil {
		pos = doc.Pos()
	}
	if index == len(decl.Specs)-1 {
		end = spec.End()
		if comment := eolComment(spec); comment != nil {
			end = comment.End()
		}
	} else {
		next := decl.Specs[index+1]
		end = next.Pos()
		if doc := docOf(next); doc != nil {
			end = doc.Pos() // the comment documents the spec that stays
		}
	}
	return []analysis.TextEdit{{Pos: pos, End: end}}
}

// repeats reports whether the specification after specAt carries no values of
// its own, and so repeats the values specAt declares. Only a const declaration
// does this, and the values it repeats have to keep their shape.
func repeats(decl *ast.GenDecl, specAt int) bool {
	if decl.Tok != token.CONST || specAt+1 >= len(decl.Specs) {
		return false
	}
	return len(decl.Specs[specAt+1].(*ast.ValueSpec).Values) == 0
}

// renumbered reports whether deleting the specification at specAt would change
// what a constant below computes. iota counts the specifications above it, so
// removing one lowers every later count, and a specification with no values of
// its own evaluates the ones it repeats.
func renumbered(decl *ast.GenDecl, specAt int) bool {
	if decl.Tok != token.CONST {
		return false
	}
	for i := specAt + 1; i < len(decl.Specs); i++ {
		if countsIota(valuesAt(decl, i)) {
			return true
		}
	}
	return false
}

// valuesAt returns the expressions the specification at index declares. A
// specification with none of its own takes the ones above it.
func valuesAt(decl *ast.GenDecl, index int) []ast.Expr {
	for i := index; i >= 0; i-- {
		if values := decl.Specs[i].(*ast.ValueSpec).Values; len(values) > 0 {
			return values
		}
	}
	return nil
}

// countsIota reports whether any of exprs names iota, whose value counts the
// specifications above it.
func countsIota(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		var found bool
		ast.Inspect(expr, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// deleteVar returns the edits that remove the name at index of the value
// specification at specAt, or the whole specification when nothing else in it
// is needed. It returns nil when no edit is safe.
//
// The name is blanked rather than removed when the specification still holds
// another name or a value whose evaluation has an effect, since both have to
// stay where they are, and when the specification below repeats these values,
// which it can only do at the same arity.
func deleteVar(tokFile *token.File, pass *analysis.Pass, file *ast.File, decl *ast.GenDecl, specAt, index int) []analysis.TextEdit {
	var (
		spec = decl.Specs[specAt].(*ast.ValueSpec)
		id   = spec.Names[index]

		// held reports whether the values cannot move without reshaping what
		// the specification below declares.
		held  = len(spec.Values) > 0 && repeats(decl, specAt)
		other = otherName(spec, id)
		fx    = effectful(pass, spec)
	)
	if !other && !fx && !held && !renumbered(decl, specAt) {
		// Nothing else needs the specification: it declares no other name
		// and evaluates nothing.
		return deleteSpec(tokFile, file, decl, specAt)
	}
	if selfRead(pass.TypesInfo, spec, id) {
		// A value here names what this name declares, and the specification
		// has to stay, so the name cannot go.
		return nil
	}
	if len(spec.Values) == 0 {
		if decl.Tok == token.CONST && specAt > 0 {
			// The specification repeats the values above it, so it has to
			// keep their count.
			return blank(id)
		}
		// Only the name carries information, so remove it and its comma.
		return dropName(spec, index)
	}
	paired := len(spec.Names) == len(spec.Values) && len(spec.Names) > 1
	if paired && !held && noEffects(pass, spec.Values[index]) {
		// The specification pairs names with values, so this pair can go.
		return dropPair(spec, index)
	}
	return blank(id)
}

// selfRead reports whether a value of spec names the object id declares, as
// the second value of var a, b = 1, a+1 names a.
func selfRead(info *types.Info, spec *ast.ValueSpec, id *ast.Ident) bool {
	obj := info.Defs[id]
	for _, value := range spec.Values {
		var found bool
		ast.Inspect(value, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && info.Uses[ident] == obj {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// dropName returns the edit that removes the name at index from spec together
// with the comma that separates it. It is only reached for a name that has
// another beside it, since a lone name with no values is deleted whole.
func dropName(spec *ast.ValueSpec, index int) []analysis.TextEdit {
	if index == len(spec.Names)-1 {
		return []analysis.TextEdit{{
			Pos: spec.Names[index-1].End(),
			End: spec.Names[index].End(),
		}}
	}
	return []analysis.TextEdit{{
		Pos: spec.Names[index].Pos(),
		End: spec.Names[index+1].Pos(),
	}}
}

// dropPair returns the edits that remove the name at index from spec and the
// value declared with it.
func dropPair(spec *ast.ValueSpec, index int) []analysis.TextEdit {
	if index == len(spec.Names)-1 {
		return []analysis.TextEdit{{
			Pos: spec.Names[index-1].End(),
			End: spec.Names[index].End(),
		}, {
			Pos: spec.Values[index-1].End(),
			End: spec.Values[index].End(),
		}}
	}
	return []analysis.TextEdit{{
		Pos: spec.Names[index].Pos(),
		End: spec.Names[index+1].Pos(),
	}, {
		Pos: spec.Values[index].Pos(),
		End: spec.Values[index+1].Pos(),
	}}
}

// blank returns the edit that replaces id with the blank identifier.
func blank(id *ast.Ident) []analysis.TextEdit {
	return []analysis.TextEdit{{
		Pos:     id.Pos(),
		End:     id.End(),
		NewText: []byte("_"),
	}}
}

// otherName reports whether spec declares a name besides id and the blank
// identifier.
func otherName(spec *ast.ValueSpec, id *ast.Ident) bool {
	for _, name := range spec.Names {
		if name != id && name.Name != "_" {
			return true
		}
	}
	return false
}

// effectful reports whether a specification carries an effect the fix has to
// keep.
func effectful(pass *analysis.Pass, spec *ast.ValueSpec) bool {
	for _, value := range spec.Values {
		if !noEffects(pass, value) {
			return true
		}
	}
	return false
}

// noEffects reports whether evaluating expr changes nothing a program can
// observe, so a fix may drop it: no code runs and nothing panics. astexpr.Safe
// answers that question. A closure that nothing calls and an allocation that
// nothing holds are droppable too, and Safe refuses both.
func noEffects(pass *analysis.Pass, expr ast.Expr) bool {
	expr = ast.Unparen(expr)
	if _, ok := expr.(*ast.FuncLit); ok {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || !allocates(pass, call) {
		return astexpr.Safe(pass, expr)
	}
	for _, arg := range call.Args {
		if !astexpr.Safe(pass, arg) {
			return false
		}
	}
	return true
}

// allocates reports whether call is a make or a new.
func allocates(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pass.TypesInfo.Uses[id].(*types.Builtin)
	if !ok {
		return false
	}
	return builtin.Name() == "make" || builtin.Name() == "new"
}

// docOf returns the doc comment of the declarations and specifications that
// can carry one.
func docOf(n ast.Node) *ast.CommentGroup {
	switch n := n.(type) {
	case *ast.FuncDecl:
		return n.Doc
	case *ast.GenDecl:
		return n.Doc
	case *ast.ValueSpec:
		return n.Doc
	case *ast.TypeSpec:
		return n.Doc
	}
	return nil
}

// eolComment returns the comment on the line a specification ends on.
func eolComment(spec ast.Spec) *ast.CommentGroup {
	switch spec := spec.(type) {
	case *ast.ValueSpec:
		return spec.Comment
	case *ast.TypeSpec:
		return spec.Comment
	}
	return nil
}

// lineOf returns the line pos is on.
func lineOf(tokFile *token.File, pos token.Pos) int {
	return tokFile.PositionFor(pos, false).Line
}
