// Package ifreturn provides an analysis.Analyzer that reports an else block
// after an if whose body only returns.
//
// Go's Code Review Comments give the shape as "indent error flow"
// (https://go.dev/wiki/CodeReviewComments#indent-error-flow). The body returns
// when the condition holds, so the else runs only when it does not. The else
// block can follow the if at the same indent, so its statements lose a level.
//
// An else-if link, an empty else block, and an else block that holds only a
// comment are not reported. An if with an init statement is not reported,
// because the else block can name a value declared there. The fix
// removes the else and its braces and outdents the block. It is withheld when
// a comment sits between the closing brace of the if body and the opening
// brace of the else block, when the block holds a raw string that spans lines,
// whose indentation is part of its value, and when moving a declaration would
// change what a later statement, deferred call, closure, or pointer observes,
// or would break the scope rules at its new position.
//
// The rule is adapted from revive's indent-error-flow. The suite's errflow
// reports the same else without a fix; this rule supplies the fix for the body
// that only returns.
package ifreturn

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ifreturn",
	Doc:  "reports an else block after an if that only returns",
	Run:  run,
}

// frame describes the function and clause an if belongs to.
type frame struct {
	params  map[string]bool
	results map[string]bool
	hidden  map[string]bool
}

// scope describes the statement list an if belongs to.
type scope struct {
	list []ast.Stmt
	item ast.Stmt
	frame
	jump bool
}

// visitor checks each statement list once and gives it its function's frame.
type visitor struct {
	pass   *analysis.Pass
	file   *ast.File
	body   *ast.BlockStmt
	hidden map[ast.Node]map[string]bool
	ctx    frame
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		hidden := hiddenNames(file)
		ast.Walk(&visitor{pass: pass, file: file, hidden: hidden}, file)
	}
	return nil, nil
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Body != nil {
			v.function(n.Type, n.Recv, n.Body)
		}
		return nil
	case *ast.FuncLit:
		v.function(n.Type, nil, n.Body)
		return nil
	case *ast.BlockStmt:
		ctx := v.ctx
		if n != v.body {
			ctx.params = nil
		}
		checkList(v.pass, v.file, n.List, ctx)
	case *ast.CaseClause:
		checkList(v.pass, v.file, n.Body, v.clause(n))
	case *ast.CommClause:
		checkList(v.pass, v.file, n.Body, v.clause(n))
	}
	return v
}

// function walks body with the frame its declarations create.
func (v *visitor) function(ft *ast.FuncType, recv *ast.FieldList, body *ast.BlockStmt) {
	child := &visitor{
		pass:   v.pass,
		file:   v.file,
		body:   body,
		hidden: v.hidden,
		ctx:    frame{params: names(ft, recv), results: results(ft)},
	}
	ast.Walk(child, body)
}

// clause returns the frame for a switch or select clause.
func (v *visitor) clause(node ast.Node) frame {
	ctx := v.ctx
	ctx.params = nil
	ctx.hidden = v.hidden[node]
	return ctx
}

// checkList checks the if statements that are direct statements of a list.
func checkList(pass *analysis.Pass, file *ast.File, list []ast.Stmt, ctx frame) {
	for _, item := range list {
		stmt, ok := unlabel(item).(*ast.IfStmt)
		if !ok || !reportable(stmt) {
			continue
		}
		block := stmt.Else.(*ast.BlockStmt)
		diagnostic := analysis.Diagnostic{
			Pos:     stmt.Else.Pos(),
			End:     block.End(),
			Message: "else follows an if whose body only returns",
		}
		s := scope{list: list, item: item, frame: ctx, jump: jumps(list)}
		if fixes, ok := dropElse(pass, file, stmt, block, s); ok {
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message:   "Drop the else",
				TextEdits: fixes,
			}}
		}
		pass.Report(diagnostic)
	}
}

// hiddenNames returns the names a clause declares for its body.
func hiddenNames(file *ast.File) map[ast.Node]map[string]bool {
	out := make(map[ast.Node]map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeSwitchStmt:
			names := assignNames(n.Assign)
			if len(names) == 0 {
				break
			}
			for _, clause := range n.Body.List {
				out[clause] = names
			}
		case *ast.CommClause:
			names := assignNames(n.Comm)
			if len(names) > 0 {
				out[n] = names
			}
		}
		return true
	})
	return out
}

// assignNames returns the names a short declaration binds.
func assignNames(stmt ast.Stmt) map[string]bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil
	}
	out := make(map[string]bool)
	for _, lhs := range assign.Lhs {
		if name, ok := lhs.(*ast.Ident); ok && name.Name != "_" {
			out[name.Name] = true
		}
	}
	return out
}

// jumps reports whether list holds a goto outside a nested function.
func jumps(list []ast.Stmt) bool {
	for _, stmt := range list {
		var found bool
		ast.Inspect(stmt, func(n ast.Node) bool {
			if found {
				return false
			}
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			branch, ok := n.(*ast.BranchStmt)
			if ok && branch.Tok == token.GOTO {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// unlabel returns the statement a label holds.
func unlabel(stmt ast.Stmt) ast.Stmt {
	for {
		label, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = label.Stmt
	}
}

// results returns the names a function declares for its results.
func results(ft *ast.FuncType) map[string]bool {
	out := make(map[string]bool)
	addNames(out, ft.Results)
	return out
}

// names returns the names a function declares for its body.
func names(ft *ast.FuncType, recv *ast.FieldList) map[string]bool {
	out := make(map[string]bool)
	addNames(out, recv)
	addNames(out, ft.TypeParams)
	addNames(out, ft.Params)
	addNames(out, ft.Results)
	if recv != nil {
		for _, field := range recv.List {
			addTypeParams(out, field.Type)
		}
	}
	return out
}

// addTypeParams adds the type parameter names in a receiver type.
func addTypeParams(out map[string]bool, expr ast.Expr) {
	switch expr := expr.(type) {
	case *ast.StarExpr:
		addTypeParams(out, expr.X)
	case *ast.ParenExpr:
		addTypeParams(out, expr.X)
	case *ast.IndexExpr:
		addIdents(out, expr.Index)
	case *ast.IndexListExpr:
		for _, index := range expr.Indices {
			addIdents(out, index)
		}
	}
}

// addIdents adds every identifier in expr to out.
func addIdents(out map[string]bool, expr ast.Expr) {
	ast.Inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name != "_" {
			out[id.Name] = true
		}
		return true
	})
}

// addNames adds the names of fields to out.
func addNames(out map[string]bool, fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.Name != "_" {
				out[name.Name] = true
			}
		}
	}
}

// reportable reports whether stmt has a body that only returns and an else
// block that can follow it.
func reportable(stmt *ast.IfStmt) bool {
	if stmt.Init != nil || stmt.Else == nil {
		return false
	}
	block, ok := stmt.Else.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return false
	}
	if len(stmt.Body.List) != 1 {
		return false
	}
	_, ok = stmt.Body.List[0].(*ast.ReturnStmt)
	return ok
}

// dropElse returns the edit that removes the else and outdents its block.
func dropElse(pass *analysis.Pass, file *ast.File, stmt *ast.IfStmt, block *ast.BlockStmt, s scope) ([]analysis.TextEdit, bool) {
	if commented(file, stmt.Body.Rbrace, block.Lbrace) {
		return nil, false
	}
	if moved(pass, block, s) {
		return nil, false
	}
	tokFile := pass.Fset.File(stmt.Pos())
	if tokFile == nil {
		return nil, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return nil, false
	}
	parent := indentAt(src, tokFile.Offset(stmt.Pos()))
	unit := indentUnit(src, tokFile.Offset(block.List[0].Pos()), parent)
	if hasRawString(block) {
		return nil, false
	}
	inner := src[tokFile.Offset(block.Lbrace)+1 : tokFile.Offset(block.Rbrace)]
	text := outdent(inner, parent, unit)
	if len(text) == 0 {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos:     stmt.Body.Rbrace + 1,
		End:     block.Rbrace + 1,
		NewText: append([]byte("\n"), text...),
	}}, true
}

// binding is one declaration in a block.
type binding struct {
	ids    []*ast.Ident
	short  bool
	assign *ast.AssignStmt
}

// moved reports whether outdenting block would move a declaration into a
// scope where its name already means something.
func moved(pass *analysis.Pass, block *ast.BlockStmt, s scope) bool {
	bindings := declared(block)
	if s.jump && (len(bindings) > 0 || blankVar(block)) {
		return true
	}
	if len(bindings) == 0 {
		return false
	}
	prior := parentObjects(pass, s.list, s.item)
	for _, b := range bindings {
		if b.moved(pass, prior, s) {
			return true
		}
		b.add(pass, prior)
	}
	return false
}

// add records the objects a moved binding puts in the enclosing scope.
func (b binding) add(pass *analysis.Pass, scope map[string]types.Object) {
	for _, id := range b.ids {
		if obj := objectOf(pass, id); obj != nil {
			scope[id.Name] = obj
		}
	}
}

// objectOf returns the object id names, whether it defines or uses it.
func objectOf(pass *analysis.Pass, id *ast.Ident) types.Object {
	if obj := pass.TypesInfo.Defs[id]; obj != nil {
		return obj
	}
	return pass.TypesInfo.Uses[id]
}

// moved reports whether one binding meets a name it cannot move.
func (b binding) moved(pass *analysis.Pass, prior map[string]types.Object, s scope) bool {
	if b.short {
		return b.movedShort(pass, prior, s)
	}
	for _, id := range b.ids {
		name := id.Name
		if s.params[name] || s.hidden[name] || shadowsResult(s, name) {
			return true
		}
		if _, ok := prior[name]; ok {
			return true
		}
		if usedAfter(pass, s.list, s.item, name) {
			return true
		}
	}
	return false
}

// movedShort reports whether a short declaration meets a name it cannot
// reassign.
func (b binding) movedShort(pass *analysis.Pass, prior map[string]types.Object, s scope) bool {
	var anyNew bool
	for _, lhs := range b.assign.Lhs {
		name, ok := lhs.(*ast.Ident)
		if !ok || name.Name == "_" {
			continue
		}
		if newName(name.Name, prior, s) {
			anyNew = true
		}
	}
	for _, lhs := range b.assign.Lhs {
		name, ok := lhs.(*ast.Ident)
		if !ok || name.Name == "_" {
			continue
		}
		if reused(pass, prior, s, name, anyNew) {
			return true
		}
	}
	return false
}

// newName reports whether a short declaration introduces name.
func newName(name string, prior map[string]types.Object, s scope) bool {
	if _, ok := prior[name]; ok {
		return false
	}
	return !s.params[name] && !s.hidden[name]
}

// reused reports whether one name of a short declaration cannot reuse its
// earlier value.
func reused(pass *analysis.Pass, prior map[string]types.Object, s scope, id *ast.Ident, anyNew bool) bool {
	name := id.Name
	if s.params[name] || s.hidden[name] || shadowsResult(s, name) {
		return true
	}
	if obj, ok := prior[name]; ok {
		if !anyNew || escapesBefore(pass, s.list, s.item, name) {
			return true
		}
		if escapesNode(pass, s.item, name) {
			return true
		}
		if !sameType(obj, objectOf(pass, id)) {
			return true
		}
	}
	return usedAfter(pass, s.list, s.item, name)
}

// sameType reports whether new can replace prior without changing its type.
func sameType(prior, next types.Object) bool {
	oldVar, ok := prior.(*types.Var)
	if !ok || oldVar.IsField() {
		return false
	}
	nextVar, ok := next.(*types.Var)
	return ok && types.Identical(oldVar.Type(), nextVar.Type())
}

// declared returns the bindings block declares directly.
func declared(block *ast.BlockStmt) []binding {
	return declaredList(block.List)
}

// blankVar reports whether block declares a blank variable directly.
func blankVar(block *ast.BlockStmt) bool {
	for _, stmt := range block.List {
		decl, ok := unlabel(stmt).(*ast.DeclStmt)
		if !ok {
			continue
		}
		gen, ok := decl.Decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			spec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range spec.Names {
				if name.Name == "_" {
					return true
				}
			}
		}
	}
	return false
}

// declaredList returns the bindings list declares directly.
func declaredList(list []ast.Stmt) (out []binding) {
	for _, stmt := range list {
		switch stmt := unlabel(stmt).(type) {
		case *ast.DeclStmt:
			decl, ok := stmt.Decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			var ids []*ast.Ident
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.Name != "_" {
							ids = append(ids, name)
						}
					}
				case *ast.TypeSpec:
					if spec.Name.Name != "_" {
						ids = append(ids, spec.Name)
					}
				}
			}
			if len(ids) > 0 {
				out = append(out, binding{ids: ids})
			}
		case *ast.AssignStmt:
			if stmt.Tok != token.DEFINE {
				continue
			}
			var ids []*ast.Ident
			for _, lhs := range stmt.Lhs {
				if name, ok := lhs.(*ast.Ident); ok && name.Name != "_" {
					ids = append(ids, name)
				}
			}
			if len(ids) > 0 {
				out = append(out, binding{ids: ids, short: true, assign: stmt})
			}
		}
	}
	return out
}

// parentObjects returns the names list declares before item.
func parentObjects(pass *analysis.Pass, list []ast.Stmt, item ast.Stmt) map[string]types.Object {
	out := make(map[string]types.Object)
	for _, stmt := range list {
		if stmt == item {
			break
		}
		switch stmt := unlabel(stmt).(type) {
		case *ast.DeclStmt:
			addObjects(pass, out, stmt)
		case *ast.AssignStmt:
			if stmt.Tok == token.DEFINE {
				addObjects(pass, out, stmt)
			}
		}
	}
	return out
}

// addObjects adds the objects stmt defines to out.
func addObjects(pass *analysis.Pass, out map[string]types.Object, stmt ast.Stmt) {
	switch stmt := unlabel(stmt).(type) {
	case *ast.DeclStmt:
		decl, ok := stmt.Decl.(*ast.GenDecl)
		if !ok {
			return
		}
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.ValueSpec:
				for _, name := range spec.Names {
					addObject(pass, out, name)
				}
			case *ast.TypeSpec:
				addObject(pass, out, spec.Name)
			}
		}
	case *ast.AssignStmt:
		if stmt.Tok != token.DEFINE {
			return
		}
		for _, lhs := range stmt.Lhs {
			if name, ok := lhs.(*ast.Ident); ok {
				addObject(pass, out, name)
			}
		}
	}
}

// addObject adds the object name defines to out.
func addObject(pass *analysis.Pass, out map[string]types.Object, name *ast.Ident) {
	if obj := pass.TypesInfo.Defs[name]; obj != nil {
		out[name.Name] = obj
	}
}

// escapesBefore reports whether a statement before item keeps a reference to
// name across the reassignment an outdented declaration would make.
func escapesBefore(pass *analysis.Pass, list []ast.Stmt, item ast.Stmt, name string) bool {
	for _, stmt := range list {
		if stmt == item {
			break
		}
		if escapesNode(pass, stmt, name) {
			return true
		}
	}
	return false
}

// escapesNode reports whether node captures or addresses name.
func escapesNode(pass *analysis.Pass, node ast.Node, name string) bool {
	var found bool
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.UnaryExpr:
			if n.Op == token.AND && referenced(pass, n.X, name, true) {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			if methodValue(pass, n, name) {
				found = true
				return false
			}
		case *ast.SliceExpr:
			if arraySlice(pass, n, name) {
				found = true
				return false
			}
		case *ast.FuncLit:
			if usesOuter(pass, n, name) {
				found = true
			}
			return false
		}
		return !found
	})
	return found
}

// shadowsResult reports whether a nested declaration hides a naked return's
// result.
func shadowsResult(s scope, name string) bool {
	return s.results[name] && !s.params[name] && nakedAfter(s.list, s.item)
}

// nakedAfter reports whether a statement after item has a naked return.
func nakedAfter(list []ast.Stmt, item ast.Stmt) bool {
	var after bool
	for _, stmt := range list {
		if stmt == item {
			after = true
			continue
		}
		if !after {
			continue
		}
		var found bool
		ast.Inspect(stmt, func(n ast.Node) bool {
			if found {
				return false
			}
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			ret, ok := n.(*ast.ReturnStmt)
			if ok && len(ret.Results) == 0 {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// usedAfter reports whether a statement after item observes name.
func usedAfter(pass *analysis.Pass, list []ast.Stmt, item ast.Stmt, name string) bool {
	var after bool
	for _, stmt := range list {
		if stmt == item {
			after = true
			continue
		}
		if after && affects(pass, stmt, name) {
			return true
		}
	}
	return false
}

// affects reports whether stmt observes name from the enclosing block.
func affects(pass *analysis.Pass, stmt ast.Stmt, name string) bool {
	stmt = unlabel(stmt)
	if declares(stmt, name) {
		return true
	}
	return usesOuter(pass, stmt, name)
}

// declares reports whether stmt declares or assigns name directly.
func declares(stmt ast.Stmt, name string) bool {
	switch stmt := stmt.(type) {
	case *ast.DeclStmt:
		decl, ok := stmt.Decl.(*ast.GenDecl)
		if !ok {
			return false
		}
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.ValueSpec:
				for _, id := range spec.Names {
					if id.Name == name {
						return true
					}
				}
			case *ast.TypeSpec:
				if spec.Name.Name == name {
					return true
				}
			}
		}
	case *ast.AssignStmt:
		for _, lhs := range stmt.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
				return true
			}
		}
	}
	return false
}

// usesOuter reports whether node uses an object declared outside node.
func usesOuter(pass *analysis.Pass, node ast.Node, name string) bool {
	var (
		skip  = selectorNames(node)
		found bool
	)
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok || skip[id] || id.Name != name {
			return true
		}
		obj := pass.TypesInfo.Uses[id]
		if obj == nil {
			return true
		}
		if _, ok := obj.(*types.Label); ok {
			return true
		}
		if v, ok := obj.(*types.Var); ok && v.IsField() {
			return true
		}
		if obj.Pos() >= node.Pos() && obj.Pos() < node.End() {
			return true
		}
		found = true
		return false
	})
	return found
}

// methodValue reports whether sel makes a method value from name.
func methodValue(pass *analysis.Pass, sel *ast.SelectorExpr, name string) bool {
	selection, ok := pass.TypesInfo.Selections[sel]
	return ok && selection.Kind() == types.MethodVal &&
		referenced(pass, sel.X, name, true)
}

// arraySlice reports whether slice takes a view of an array named name.
func arraySlice(pass *analysis.Pass, slice *ast.SliceExpr, name string) bool {
	if !referenced(pass, slice.X, name, true) {
		return false
	}
	typ := pass.TypesInfo.TypeOf(slice.X)
	return typ != nil && arrayType(typ)
}

// arrayType reports whether typ is an array or a type parameter.
func arrayType(typ types.Type) bool {
	typ = types.Unalias(typ)
	if _, ok := typ.(*types.TypeParam); ok {
		return true
	}
	_, ok := typ.Underlying().(*types.Array)
	return ok
}

// referenced reports whether node names name. A use is any reference; a
// definition counts only when defs is true.
func referenced(pass *analysis.Pass, node ast.Node, name string, defs bool) bool {
	var (
		skip  = selectorNames(node)
		found bool
	)
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		id, ok := n.(*ast.Ident)
		if ok && !skip[id] && id.Name == name && refers(pass, id, defs) {
			found = true
			return false
		}
		return true
	})
	return found
}

// selectorNames returns the member names of selectors under node.
func selectorNames(node ast.Node) map[*ast.Ident]bool {
	out := make(map[*ast.Ident]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			out[sel.Sel] = true
		}
		return true
	})
	return out
}

// refers reports whether id names an object a declaration could shadow.
func refers(pass *analysis.Pass, id *ast.Ident, defs bool) bool {
	obj := pass.TypesInfo.Uses[id]
	if obj == nil && defs {
		obj = pass.TypesInfo.Defs[id]
	}
	if obj == nil {
		return false
	}
	if _, ok := obj.(*types.Label); ok {
		return false
	}
	if v, ok := obj.(*types.Var); ok && v.IsField() {
		return false
	}
	return true
}

// commented reports whether a comment overlaps the half-open range lo to hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() >= hi {
			return false
		}
		if group.End() > lo {
			return true
		}
	}
	return false
}

// indentAt returns the whitespace that starts the line holding off.
func indentAt(src []byte, off int) []byte {
	start := off
	for start > 0 && src[start-1] != '\n' {
		start--
	}
	end := start
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return src[start:end]
}

// indentUnit returns the indentation the block adds to parent.
func indentUnit(src []byte, off int, parent []byte) []byte {
	inner := indentAt(src, off)
	if !bytes.HasPrefix(inner, parent) {
		return nil
	}
	return inner[len(parent):]
}

// hasRawString reports whether block holds a raw string that spans lines.
func hasRawString(block *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(block, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return !found
		}
		if strings.Contains(lit.Value, "\n") {
			found = true
			return false
		}
		return !found
	})
	return found
}

// outdent removes one indent unit from each line of inner and puts parent at
// the start of the first line.
func outdent(inner, parent, unit []byte) []byte {
	lines := strings.Split(strings.TrimSpace(string(inner)), "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.TrimPrefix(lines[i], string(unit))
	}
	text := strings.Join(lines, "\n")
	if text == "" {
		return nil
	}
	return append(bytes.Clone(parent), text...)
}
