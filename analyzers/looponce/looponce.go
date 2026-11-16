// Package looponce provides an analysis.Analyzer that reports a loop that
// runs at most once.
//
// The For statements section of the spec says a for statement evaluates its
// condition before each iteration (https://go.dev/ref/spec#For_statements). A
// body that ends in a return or a break that ends the loop ends on its first
// iteration, so the condition is not evaluated again.
//
// The rule reports a loop whose body has at least two statements, one of them
// a control statement at the top level, and ends in a return or a break that
// ends the loop. It inspects only the top-level statements of the body to find
// the control statement and the exit; a nested loop, switch, or select is not
// followed to see whether it finishes.
//
// A body of one statement is left alone, since the rule requires the control
// statement and the exit as separate top-level statements. A range body of one
// statement is the first-element idiom. A body with no top-level control
// statement is left alone. A goto in the loop body, other than one inside a
// nested function literal, suppresses the report, since it may reach the exit
// or skip it. A continue suppresses the report when it repeats the checked
// loop, either without a label or with a label that names the loop. The README
// catalogs every check, exemption, and fix.
//
// A range over a map is left alone, since one pass is a plain way to take an
// arbitrary element. A range over a function is left alone, since the rule
// does not inspect the iterator. A range over a type parameter is left alone
// without reading its type set, so this includes a type parameter whose set is
// only slices.
//
// Fixable: a for loop becomes a block, or an if when it has a condition and no
// init. An init statement becomes the first statement of the new block, and a
// condition moves into an if. The body's statements stay in a block. A block
// that declares nothing is removed by unnecessaryblock on the next fix round.
// An unlabeled final break is removed, and a post statement is dropped, since
// the body leaves before it can run. The fix is withheld when a break targets
// the loop anywhere other than an unlabeled final break, when a comment sits
// in the part of the loop that the edit replaces or on an unlabeled final
// break, or when dropping the post would leave a variable declared in the
// function body, an import name, or an object from a dot import with no reader
// outside another loop post. A range loop is report only, since a replacement
// would need a guard and a first-value expression that differs for each kind
// of rangeable value.
//
// The rule is adapted from staticcheck's SA4004.
package looponce

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "looponce",
	Doc:  "reports a loop that runs at most once",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Body != nil {
					checkFunc(pass, file, n.Body)
				}
				return false
			case *ast.FuncLit:
				checkFunc(pass, file, n.Body)
				return false
			}
			return true
		})
	}
	return nil, nil
}

// Labels in a function literal body are local to it, so the body is checked
// separately.
func checkFunc(pass *analysis.Pass, file *ast.File, body *ast.BlockStmt) {
	labels := labelIndex(body)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			checkFunc(pass, file, n.Body)
			return false
		case *ast.ForStmt:
			checkLoop(pass, file, n, n.Body, labels)
		case *ast.RangeStmt:
			if !exemptRange(pass, n.X) {
				checkLoop(pass, file, n, n.Body, labels)
			}
		}
		return true
	})
}

func checkLoop(pass *analysis.Pass, file *ast.File, loop ast.Node, body *ast.BlockStmt, labels map[string]*ast.LabeledStmt) {
	if body == nil || len(body.List) < 2 || !branches(body) {
		return
	}
	if !exits(body.List[len(body.List)-1]) {
		return
	}
	if !leaves(body, labels, loop) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     loop.Pos(),
		Message: "loop runs at most once",
	}
	if stmt, ok := loop.(*ast.ForStmt); ok {
		if edits, ok := unwrap(pass, file, stmt, labels); ok {
			message := "Replace the loop with a block"
			if stmt.Init == nil && stmt.Cond != nil {
				message = "Replace the loop with an if"
			}
			diag.SuggestedFixes = []analysis.SuggestedFix{{
				Message:   message,
				TextEdits: edits,
			}}
		}
	}
	pass.Report(diag)
}

func branches(body *ast.BlockStmt) bool {
	for _, stmt := range body.List {
		switch unlabel(stmt).(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			return true
		}
	}
	return false
}

// A return and a break both leave the loop.
func exits(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok == token.BREAK
	}
	return false
}

// A goto can skip the exit, and a continue that repeats the loop reaches the
// condition again.
func leaves(body *ast.BlockStmt, labels map[string]*ast.LabeledStmt, loop ast.Node) bool {
	var (
		ok    = true
		stack = []ast.Node{loop}
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if !ok {
			return false
		}
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		if branch, isBranch := n.(*ast.BranchStmt); isBranch {
			switch branch.Tok {
			case token.GOTO:
				ok = false
				return false
			case token.CONTINUE:
				target := enclosing(stack)
				if branch.Label != nil {
					labeled := labels[branch.Label.Name]
					if labeled == nil {
						break
					}
					target = labeled.Stmt
				}
				if target == loop {
					ok = false
					return false
				}
			}
		}
		stack = append(stack, n)
		return true
	})
	return ok
}

// enclosing returns the innermost loop in stack.
func enclosing(stack []ast.Node) ast.Node {
	for _, s := range slices.Backward(stack) {
		switch s.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return s
		}
	}
	return nil
}

// A map has no defined order, a function's range is not inspected, and a type
// parameter's type set is not read.
func exemptRange(pass *analysis.Pass, x ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(x)
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return true
	}
	switch t.Underlying().(type) {
	case *types.Map, *types.Signature:
		return true
	}
	return false
}

// Labels inside a function literal belong to that function.
func labelIndex(body *ast.BlockStmt) map[string]*ast.LabeledStmt {
	labels := make(map[string]*ast.LabeledStmt)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n
		}
		return true
	})
	return labels
}

// A label in front of a statement does not change what the statement is.
func unlabel(stmt ast.Stmt) ast.Stmt {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = labeled.Stmt
	}
}

// A condition becomes an if; an init statement stays in the outer block.
func unwrap(pass *analysis.Pass, file *ast.File, loop *ast.ForStmt, labels map[string]*ast.LabeledStmt) (edits []analysis.TextEdit, ok bool) {
	if loop.Post != nil && postNeeded(pass, file, loop.Post) {
		return nil, false
	}
	tail := tailBreak(loop.Body)
	if breaksLoop(loop.Body, labels, loop, tail) {
		return nil, false
	}
	switch {
	case loop.Init == nil && loop.Cond == nil:
		edits, ok = headerEdit(file, loop)
		if ok && tail != nil {
			if lineCommented(pass, file, tail.Pos()) {
				return nil, false
			}
			edits = append(edits, breakEdit(pass, tail))
		}
	case loop.Init == nil:
		return ifEdit(pass, file, loop, tail)
	default:
		edits, ok = blockEdit(pass, file, loop, tail)
	}
	return edits, ok
}

// lineCommented reports whether a comment sits on the line at pos.
func lineCommented(pass *analysis.Pass, file *ast.File, pos token.Pos) bool {
	tok := pass.Fset.File(pos)
	if tok == nil {
		return false
	}
	src, err := pass.ReadFile(tok.Name())
	if err != nil {
		return false
	}
	offset := tok.Offset(pos)
	start := source.Start(src, offset)
	end := source.End(src, offset)
	return commented(file.Comments, tok.Pos(start), tok.Pos(end))
}

// breakEdit removes the line holding a final break when nothing else is on it.
func breakEdit(pass *analysis.Pass, branch *ast.BranchStmt) analysis.TextEdit {
	var (
		edit = analysis.TextEdit{Pos: branch.Pos(), End: branch.End()}
		tok  = pass.Fset.File(branch.Pos())
	)
	if tok == nil {
		return edit
	}
	src, err := pass.ReadFile(tok.Name())
	if err != nil {
		return edit
	}
	start := tok.Offset(branch.Pos())
	end := tok.Offset(branch.End())
	if !source.Alone(src, start, end) {
		return edit
	}
	return analysis.TextEdit{
		Pos: tok.Pos(source.Start(src, start)),
		End: tok.Pos(source.End(src, end)),
	}
}

// tailBreak returns an unlabeled break that is the last statement of body.
func tailBreak(body *ast.BlockStmt) *ast.BranchStmt {
	if body == nil || len(body.List) == 0 {
		return nil
	}
	branch, ok := body.List[len(body.List)-1].(*ast.BranchStmt)
	if !ok || branch.Tok != token.BREAK || branch.Label != nil {
		return nil
	}
	return branch
}

// headerEdit drops a header with no init or condition. Any post is dropped.
func headerEdit(file *ast.File, loop *ast.ForStmt) ([]analysis.TextEdit, bool) {
	if commented(file.Comments, loop.Pos(), loop.Body.Lbrace) {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos: loop.Pos(),
		End: loop.Body.Lbrace,
	}}, true
}

// blockEdit wraps the init statement and the condition or body in a block.
func blockEdit(pass *analysis.Pass, file *ast.File, loop *ast.ForStmt, tail *ast.BranchStmt) ([]analysis.TextEdit, bool) {
	if commented(file.Comments, loop.Pos(), loop.End()) {
		return nil, false
	}
	var (
		body = without(loop.Body, tail)
		list []ast.Stmt
	)
	if loop.Init != nil {
		list = append(list, loop.Init)
	}
	var last ast.Stmt = body
	if loop.Cond != nil {
		last = &ast.IfStmt{Cond: loop.Cond, Body: body}
	}
	list = append(list, last)
	block := &ast.BlockStmt{
		Lbrace: loop.Body.Lbrace,
		Rbrace: loop.Body.Rbrace,
		List:   list,
	}
	text, ok := formatted(pass, loop, block)
	if !ok {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos:     loop.Pos(),
		End:     loop.End(),
		NewText: text,
	}}, true
}

// ifEdit turns a loop with a condition and no init into an if.
func ifEdit(pass *analysis.Pass, file *ast.File, loop *ast.ForStmt, tail *ast.BranchStmt) ([]analysis.TextEdit, bool) {
	if commented(file.Comments, loop.Pos(), loop.End()) {
		return nil, false
	}
	var (
		body     = without(loop.Body, tail)
		stmt     = &ast.IfStmt{Cond: loop.Cond, Body: body}
		text, ok = formatted(pass, loop, stmt)
	)
	if !ok {
		return nil, false
	}
	return []analysis.TextEdit{{
		Pos:     loop.Pos(),
		End:     loop.End(),
		NewText: text,
	}}, true
}

// without returns body without tail when tail is present.
func without(body *ast.BlockStmt, tail *ast.BranchStmt) *ast.BlockStmt {
	if tail == nil {
		return body
	}
	inner := *body
	inner.List = inner.List[:len(inner.List)-1]
	inner.Rbrace = tail.Pos()
	return &inner
}

// formatted prints node at the indentation of loop.
func formatted(pass *analysis.Pass, loop *ast.ForStmt, node ast.Node) ([]byte, bool) {
	var buf bytes.Buffer
	if err := format.Node(&buf, pass.Fset, node); err != nil {
		return nil, false
	}
	text := strings.TrimRight(buf.String(), "\n")
	if indent := indentation(pass, loop.Pos()); indent != "" {
		lines := strings.Split(text, "\n")
		for i := 1; i < len(lines); i++ {
			if lines[i] != "" {
				lines[i] = indent + lines[i]
			}
		}
		text = strings.Join(lines, "\n")
	}
	return []byte(text), true
}

// The indentation of pos is the leading whitespace on its line.
func indentation(pass *analysis.Pass, pos token.Pos) string {
	tok := pass.Fset.File(pos)
	if tok == nil {
		return ""
	}
	src, err := pass.ReadFile(tok.Name())
	if err != nil {
		return ""
	}
	offset := tok.Offset(pos)
	start := bytes.LastIndexByte(src[:offset], '\n') + 1
	line := src[start:offset]
	var end int
	for end < len(line) && (line[end] == ' ' || line[end] == '\t') {
		end++
	}
	return string(line[:end])
}

// A break leaves the loop when it has no label and its innermost breakable
// statement is the loop, or when its label names the loop.
func breaksLoop(body *ast.BlockStmt, labels map[string]*ast.LabeledStmt, loop ast.Node, ignore *ast.BranchStmt) bool {
	var (
		found bool
		stack = []ast.Node{loop}
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if found {
			return false
		}
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		branch, isBranch := n.(*ast.BranchStmt)
		if isBranch && branch == ignore {
			return false
		}
		if isBranch && branch.Tok == token.BREAK {
			target := enclosingBreakable(stack)
			if branch.Label != nil {
				labeled := labels[branch.Label.Name]
				if labeled == nil {
					return true
				}
				target = labeled.Stmt
			}
			if target == loop {
				found = true
				return false
			}
		}
		stack = append(stack, n)
		return true
	})
	return found
}

// enclosingBreakable returns the innermost loop, switch, type switch, or
// select in stack.
func enclosingBreakable(stack []ast.Node) ast.Node {
	for _, s := range slices.Backward(stack) {
		switch s.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
			return s
		}
	}
	return nil
}

// A post statement cannot be dropped when that would leave a variable declared
// in the function body, an import name, or an object from a dot import with no
// reader outside another loop post.
func postNeeded(pass *analysis.Pass, file *ast.File, post ast.Stmt) bool {
	var (
		dots   = dotImports(file)
		writes = writeOnly(file)
		needed bool
	)
	ast.Inspect(post, func(n ast.Node) bool {
		if needed {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		object := pass.TypesInfo.ObjectOf(id)
		if object == nil {
			return true
		}
		var neededObject bool
		switch obj := object.(type) {
		case *types.PkgName:
			neededObject = true
		case *types.Var:
			neededObject = local(pass, obj)
		}
		if pkg := object.Pkg(); pkg != nil && pkg != pass.Pkg {
			neededObject = neededObject || dots[pkg.Path()]
		}
		if neededObject && !usedOutside(pass, file, object, post, writes) {
			needed = true
			return false
		}
		return true
	})
	return needed
}

// local reports whether v is a variable declared in a function body.
func local(pass *analysis.Pass, v *types.Var) bool {
	if v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
		return false
	}
	return !parameter(pass, v)
}

// parameter reports whether v is a parameter, result, or receiver.
func parameter(pass *analysis.Pass, v *types.Var) bool {
	for _, file := range pass.Files {
		var found bool
		ast.Inspect(file, func(n ast.Node) bool {
			if found {
				return false
			}
			var fields []*ast.FieldList
			switch n := n.(type) {
			case *ast.FuncDecl:
				fields = []*ast.FieldList{
					n.Recv, n.Type.Params,
					n.Type.Results,
				}
			case *ast.FuncLit:
				fields = []*ast.FieldList{n.Type.Params, n.Type.Results}
			default:
				return true
			}
			for _, list := range fields {
				if list == nil {
					continue
				}
				for _, field := range list.List {
					for _, name := range field.Names {
						if pass.TypesInfo.ObjectOf(name) == v {
							found = true
							return false
						}
					}
				}
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// dotImports returns the paths imported with a dot import in file.
func dotImports(file *ast.File) map[string]bool {
	paths := make(map[string]bool)
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		if path, err := strconv.Unquote(spec.Path.Value); err == nil {
			paths[path] = true
		}
	}
	return paths
}

// writeOnly returns the positions of identifiers that only receive a value.
func writeOnly(file *ast.File) map[token.Pos]bool {
	out := make(map[token.Pos]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
				return true
			}
			for _, expr := range n.Lhs {
				if id, ok := ast.Unparen(expr).(*ast.Ident); ok {
					out[id.Pos()] = true
				}
			}
		case *ast.RangeStmt:
			if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
				return true
			}
			for _, expr := range []ast.Expr{n.Key, n.Value} {
				if id, ok := expr.(*ast.Ident); ok {
					out[id.Pos()] = true
				}
			}
		}
		return true
	})
	return out
}

// usedOutside reports whether obj has a read outside post and outside every
// loop post in file.
func usedOutside(pass *analysis.Pass, file *ast.File, obj types.Object, post ast.Stmt, writes map[token.Pos]bool) bool {
	for id, used := range pass.TypesInfo.Uses {
		if used != obj {
			continue
		}
		if id.Pos() < file.Pos() || id.Pos() >= file.End() {
			continue
		}
		if id.Pos() >= post.Pos() && id.End() <= post.End() {
			continue
		}
		if inPost(file, id.Pos()) || writes[id.Pos()] {
			continue
		}
		return true
	}
	return false
}

// inPost reports whether pos is in the post statement of a for loop in file.
func inPost(file *ast.File, pos token.Pos) bool {
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		loop, ok := n.(*ast.ForStmt)
		if ok && loop.Post != nil {
			if loop.Post.Pos() <= pos && pos <= loop.Post.End() {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// commented reports whether a comment overlaps the text from pos to end.
func commented(groups []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range groups {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
