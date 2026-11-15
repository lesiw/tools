// Package typeassertchain provides an analysis.Analyzer that reports a chain
// of type assertions on one value.
//
// A chain repeats the assertion once per link. A type switch states the
// expression once and puts each type in a case.
//
// The spec rewrites a type switch as an if-else chain and binds the expression
// once. A chain that asserts the expression itself evaluates it once per link
// tried. That is why a chain over a call or a receive is not reported.
//
// The fix is offered when the rewrite is the same program, compiles in place,
// and keeps each comment; the README lists the shapes that report without one.
//
// Like go-critic's typeAssertChain.
package typeassertchain

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "typeassertchain",
	Doc:  "reports a chain of type assertions on one value",
	Run:  run,
}

// link is one if of a chain: the assertion it makes and the names it binds.
type link struct {
	stmt   *ast.IfStmt
	assert *ast.TypeAssertExpr
	value  *ast.Ident
	result *ast.Ident
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tokFile := pass.Fset.File(file.FileStart)
		src, err := pass.ReadFile(tokFile.Name())
		if err != nil {
			return nil, err
		}
		nested := nestedIfs(file)
		ast.Inspect(file, func(n ast.Node) bool {
			head, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			if _, skip := nested[head]; skip {
				return true
			}
			links, name := chain(pass, file, tokFile, src, head)
			if links != nil {
				pass.Report(diagnostic(pass, file, tokFile, src, links, name))
			}
			return true
		})
	}
	return nil, nil
}

// nestedIfs returns the if statements that are not reported at all: those that
// are another if's else, since an else takes an if or a block and a switch
// cannot stand where an else-if link's header is.
func nestedIfs(file *ast.File) map[*ast.IfStmt]struct{} {
	nested := make(map[*ast.IfStmt]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		if next, ok := stmt.Else.(*ast.IfStmt); ok {
			nested[next] = struct{}{}
		}
		return true
	})
	return nested
}

// chain returns the links of the chain that starts at head, in source order,
// or nil when the chain has no type switch form. The links assert one
// expression against types no two of them share, and give one name to bind.
// The chain ends with no else or with a plain else block.
func chain(pass *analysis.Pass, file *ast.File, tokFile *token.File, src []byte, head *ast.IfStmt) ([]link, string) {
	first, ok := assertion(pass, head)
	if !ok || !pure(pass, first.assert.X) {
		return nil, ""
	}
	links := []link{first}
	seen := []types.Type{pass.TypesInfo.TypeOf(first.assert.Type)}
	for {
		stmt, ok := links[len(links)-1].stmt.Else.(*ast.IfStmt)
		if !ok {
			break
		}
		next, ok := assertion(pass, stmt)
		if !ok {
			return nil, ""
		}
		if !sameExpr(pass, tokFile, src, next.assert.X, first.assert.X) {
			return nil, ""
		}
		typ := pass.TypesInfo.TypeOf(next.assert.Type)
		if hasType(seen, typ) {
			return nil, ""
		}
		seen = append(seen, typ)
		links = append(links, next)
	}
	if len(links) < 2 {
		return nil, ""
	}
	name, oneName := guardOf(links)
	if !oneName || namesGuard(pass, links, name) {
		return nil, ""
	}
	if breaksOut(file, links) || readsResult(pass, links) {
		return nil, ""
	}
	if readsElseValue(pass, links) {
		return nil, ""
	}
	return links, name
}

// guardOf returns the name the switch binds, and whether the links give one. A
// link that binds the blank identifier contributes no name, and two links that
// bind different names leave no name to take.
func guardOf(links []link) (string, bool) {
	var name string
	for _, link := range links {
		if link.value.Name == "_" {
			continue
		}
		if name != "" && link.value.Name != name {
			return "", false
		}
		name = link.value.Name
	}
	return name, true
}

// namesGuard reports whether a link that binds the blank identifier names the
// guard name in its body, where the switch would bind it.
func namesGuard(pass *analysis.Pass, links []link, name string) bool {
	if name == "" {
		return false
	}
	for _, link := range links {
		if link.value.Name != "_" {
			continue
		}
		var found bool
		ast.Inspect(link.stmt.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok && id.Name == name && shadowed(pass, link.stmt.Body, id) {
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

// shadowed reports whether id would resolve to the binding a case clause
// makes. A field name is read as a selector, so a binding does not shadow it,
// and a declaration inside the body shadows the binding instead.
func shadowed(pass *analysis.Pass, body *ast.BlockStmt, id *ast.Ident) bool {
	object := pass.TypesInfo.ObjectOf(id)
	if object == nil {
		return false
	}
	if field, ok := object.(*types.Var); ok && field.IsField() {
		return false
	}
	return object.Pos() < body.Pos() || object.Pos() >= body.End()
}

// sameExpr reports whether a and b are one expression over one set of objects.
// The source text alone is not enough: each link binds its value in the scope
// the next link's expression is written in, so a later link can assert a
// different object that reads the same.
func sameExpr(pass *analysis.Pass, tokFile *token.File, src []byte, a, b ast.Expr) bool {
	if text(tokFile, src, a) != text(tokFile, src, b) {
		return false
	}
	return slices.Equal(idents(pass, a), idents(pass, b))
}

// idents returns the objects n names, in the order n names them.
func idents(pass *analysis.Pass, n ast.Node) []types.Object {
	var out []types.Object
	ast.Inspect(n, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if object := pass.TypesInfo.ObjectOf(id); object != nil {
			out = append(out, object)
		}
		return true
	})
	return out
}

// hasType reports whether typ is one of the types the chain already asserts.
// A type switch rejects a repeated case type, and a type written through an
// alias is the type it aliases.
func hasType(seen []types.Type, typ types.Type) bool {
	if typ == nil {
		return false
	}
	return slices.ContainsFunc(seen, func(t types.Type) bool {
		return types.Identical(t, typ)
	})
}

// assertion returns the link stmt makes, if it is the two-result form whose
// condition tests the ok result.
func assertion(pass *analysis.Pass, stmt *ast.IfStmt) (link, bool) {
	assign, ok := stmt.Init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return link{}, false
	}
	if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return link{}, false
	}
	value, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return link{}, false
	}
	result, ok := assign.Lhs[1].(*ast.Ident)
	if !ok {
		return link{}, false
	}
	assert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || assert.Type == nil {
		return link{}, false
	}
	cond, ok := stmt.Cond.(*ast.Ident)
	if !ok {
		return link{}, false
	}
	if pass.TypesInfo.ObjectOf(cond) != pass.TypesInfo.ObjectOf(result) {
		return link{}, false
	}
	return link{stmt: stmt, assert: assert, value: value, result: result}, true
}

// pure reports whether evaluating e again is the same as evaluating it once.
// A chain evaluates its expression once per link tried, and a type switch
// evaluates the guard once, so an expression that calls or receives is not
// pure. A conversion is a call expression in the tree, and converting the same
// value again changes nothing; an assertion inside the expression either
// panics at the first read or yields the same value.
func pure(pass *analysis.Pass, e ast.Expr) bool {
	var impure bool
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if !pass.TypesInfo.Types[n.Fun].IsType() {
				impure = true
			}
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				impure = true
			}
		}
		return !impure
	})
	return !impure
}

// diagnostic returns the report for one chain, with the edit that writes it as
// a type switch when the rewrite is the same program.
func diagnostic(pass *analysis.Pass, file *ast.File, tokFile *token.File, src []byte, links []link, name string) analysis.Diagnostic {
	diag := analysis.Diagnostic{
		Pos:     links[0].stmt.Pos(),
		End:     links[len(links)-1].stmt.End(),
		Message: "if-else chain of type assertions; use a type switch",
	}
	if soleElse(file, links[0].stmt) || redeclares(pass, links, name) {
		return diag
	}
	// A switch with no name of its own leaves each case to assert the value
	// again, which typeswitchvar reports.
	if name == "" && finalElse(links) == nil {
		if assertsAgain(pass, tokFile, src, links) {
			return diag
		}
	}
	lead, ok := indent(tokFile, src, links[0].stmt)
	if !ok {
		return diag
	}
	textEdits, ok := switchEdits(tokFile, src, links, lead, name)
	if !ok || losesComment(file, textEdits) {
		return diag
	}
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message:   "Write a type switch",
		TextEdits: textEdits,
	}}
	return diag
}

// redeclares reports whether a body declares the name the guard variable would
// take. A case clause and a default clause declare the guard in the implicit
// block their statements land in, so a body that declares the name in its own
// block would collide with it.
func redeclares(pass *analysis.Pass, links []link, name string) bool {
	for _, block := range branches(links) {
		if declares(pass, block, name) {
			return true
		}
	}
	return false
}

// match is a predicate over one node.
type match func(ast.Node) bool

// inBlock reports whether match finds a node in block.
func inBlock(block *ast.BlockStmt, m match) bool {
	var found bool
	ast.Inspect(block, func(n ast.Node) bool {
		if m(n) {
			found = true
		}
		return !found
	})
	return found
}

// branches returns the blocks the rewrite moves: every link's body, and the
// block a final else becomes.
func branches(links []link) []*ast.BlockStmt {
	out := make([]*ast.BlockStmt, 0, len(links)+1)
	for _, link := range links {
		out = append(out, link.stmt.Body)
	}
	if block := finalElse(links); block != nil {
		out = append(out, block)
	}
	return out
}

// finalElse returns the block a chain's else becomes, or nil when the chain
// ends with no else.
func finalElse(links []link) *ast.BlockStmt {
	var (
		last     = links[len(links)-1]
		block, _ = last.stmt.Else.(*ast.BlockStmt)
	)
	return block
}

// declares reports whether block declares name in its own scope. Lookup would
// also find the name in an enclosing scope, where a link's own binding sits.
func declares(pass *analysis.Pass, block *ast.BlockStmt, name string) bool {
	scope := pass.TypesInfo.Scopes[block]
	return scope != nil && slices.Contains(scope.Names(), name)
}

// soleElse reports whether stmt is the only statement of an else block, where
// elseif removes the braces this fix's header rewrite lands beside, leaving an
// else switch that does not parse.
func soleElse(file *ast.File, stmt ast.Node) bool {
	path, _ := astutil.PathEnclosingInterval(file, stmt.Pos(), stmt.Pos())
	for i, node := range path {
		block, ok := node.(*ast.BlockStmt)
		// stmt must be the block's own statement, not something nested in
		// a statement the block holds.
		if !ok || i == 0 || path[i-1] != stmt || len(block.List) != 1 {
			continue
		}
		if i+1 < len(path) {
			parent, ok := path[i+1].(*ast.IfStmt)
			if ok && parent.Else == block {
				return true
			}
		}
	}
	return false
}

// assertsAgain reports whether every branch asserts the value to the type its
// own case would bind, which typeswitchvar reports in the switch this fix
// writes. It asks only about a guard it names, as typeswitchvar does.
func assertsAgain(pass *analysis.Pass, tokFile *token.File, src []byte, links []link) bool {
	if _, ok := links[0].assert.X.(*ast.Ident); !ok {
		return false
	}
	for _, link := range links {
		var found bool
		ast.Inspect(link.stmt.Body, func(n ast.Node) bool {
			assert, ok := n.(*ast.TypeAssertExpr)
			if !ok || assert.Type == nil {
				return true
			}
			if !sameExpr(pass, tokFile, src, assert.X, link.assert.X) {
				return true
			}
			typ := pass.TypesInfo.TypeOf(assert.Type)
			if types.Identical(pass.TypesInfo.TypeOf(link.assert.Type), typ) {
				found = true
			}
			return !found
		})
		if !found {
			return false
		}
	}
	return true
}

// readsResult reports whether a branch names an ok result, which a type switch
// does not bind. A result can be read from any branch, since an ok name can
// differ from link to link and the final else is in the scope of every link's
// init.
func readsResult(pass *analysis.Pass, links []link) bool {
	dropped := make([]types.Object, 0, len(links))
	for _, link := range links {
		dropped = append(dropped, pass.TypesInfo.ObjectOf(link.result))
	}
	for _, block := range branches(links) {
		if inBlock(block, names(pass, dropped)) {
			return true
		}
	}
	return false
}

// readsElseValue reports whether the final else names a link's value. Every
// link has failed by then, so each binding is the zero value of its type,
// where a default clause binds the name to the value and type of the asserted
// expression.
func readsElseValue(pass *analysis.Pass, links []link) bool {
	block := finalElse(links)
	if block == nil {
		return false
	}
	values := make([]types.Object, 0, len(links))
	for _, link := range links {
		values = append(values, pass.TypesInfo.ObjectOf(link.value))
	}
	return inBlock(block, names(pass, values))
}

// breaksOut reports whether a branch breaks out of a statement around the
// chain. The type switch would hold the break, so the branch would leave a
// case instead.
func breaksOut(file *ast.File, links []link) bool {
	for _, block := range branches(links) {
		leaves := func(n ast.Node) bool {
			branch, ok := n.(*ast.BranchStmt)
			return ok && branch.Tok == token.BREAK && branch.Label == nil &&
				escapes(file, block, branch)
		}
		if inBlock(block, leaves) {
			return true
		}
	}
	return false
}

// escapes reports whether branch leaves block, so that a type switch would
// hold the break instead.
func escapes(file *ast.File, block *ast.BlockStmt, branch *ast.BranchStmt) bool {
	path, _ := astutil.PathEnclosingInterval(file, branch.Pos(), branch.End())
	for _, node := range path {
		if node == block {
			return true
		}
		switch node.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		}
	}
	return true
}

// names reports whether a node names any of objects.
func names(pass *analysis.Pass, objects []types.Object) match {
	return func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		return ok && slices.Contains(objects, pass.TypesInfo.ObjectOf(id))
	}
}

// binding returns the text that declares the switch's variable, which is empty
// when the links give no name: a type switch rejects a blank on the left of
// :=, and a chain that discards the value needs no variable.
func binding(name string) string {
	if name == "" {
		return ""
	}
	return name + " := "
}

// switchEdits returns the edits that rewrite the chain as a type switch: a
// link's header, and the brace that closes the body before it, since a case
// clause holds its statements with no braces of its own, and the final else
// becomes default.
func switchEdits(tokFile *token.File, src []byte, links []link, lead, name string) ([]analysis.TextEdit, bool) {
	out := make([]analysis.TextEdit, 0, len(links)+1)
	for i, link := range links {
		if i == 0 {
			out = append(out, analysis.TextEdit{
				Pos: link.stmt.Pos(),
				End: link.stmt.Body.Lbrace + 1,
				NewText: []byte("switch " + binding(name) +
					text(tokFile, src, link.assert.X) + ".(type) {\n" +
					lead + "case " +
					text(tokFile, src, link.assert.Type) + ":",
				),
			})
			continue
		}
		start, ok := lineStart(tokFile, src, links[i-1].stmt.Body.Rbrace)
		if !ok {
			return nil, false
		}
		out = append(out, analysis.TextEdit{
			Pos: tokFile.Pos(start),
			End: link.stmt.Body.Lbrace + 1,
			NewText: []byte(lead + "case " +
				text(tokFile, src, link.assert.Type) + ":",
			),
		})
	}
	block := finalElse(links)
	if block == nil {
		return out, true
	}
	start, ok := lineStart(tokFile, src, links[len(links)-1].stmt.Body.Rbrace)
	if !ok {
		return nil, false
	}
	out = append(out, analysis.TextEdit{
		Pos:     tokFile.Pos(start),
		End:     block.Lbrace + 1,
		NewText: []byte(lead + "default:"),
	})
	return out, true
}

// losesComment reports whether an edit covers a comment, which the rewrite
// would drop.
func losesComment(file *ast.File, edits []analysis.TextEdit) bool {
	for _, comment := range file.Comments {
		for _, edit := range edits {
			if comment.Pos() >= edit.Pos && comment.Pos() < edit.End {
				return true
			}
		}
	}
	return false
}

// text returns node's source, so the rewrite keeps it as written.
func text(tokFile *token.File, src []byte, node ast.Node) string {
	return string(src[tokFile.Offset(node.Pos()):tokFile.Offset(node.End())])
}

// indent returns the whitespace that leads stmt's line, which is the column
// gofmt puts the case labels in.
func indent(tokFile *token.File, src []byte, stmt *ast.IfStmt) (string, bool) {
	start, ok := lineStart(tokFile, src, stmt.Pos())
	if !ok {
		return "", false
	}
	return string(src[start:tokFile.Offset(stmt.Pos())]), true
}

// lineStart returns the offset of the first byte of pos's line, when the line
// holds nothing but whitespace before pos. A statement that shares its line
// with code before it has no indentation to take and no line to replace.
func lineStart(tokFile *token.File, src []byte, pos token.Pos) (int, bool) {
	var (
		off  = tokFile.Offset(pos)
		line = tokFile.Offset(tokFile.LineStart(tokFile.Line(pos)))
	)
	if strings.TrimSpace(string(src[line:off])) != "" {
		return 0, false
	}
	return line, true
}
