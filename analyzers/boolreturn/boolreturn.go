// Package boolreturn provides an analysis.Analyzer that reports two returns of
// opposite boolean constants under an if.
//
// The condition is the value the function returns. Testing it, then spelling
// the answer in both branches, says the same thing twice:
//
//	if inRange(n) {
//		return true
//	}
//	return false
//
// That is return inRange(n). If the branch that returns false comes first,
// the condition is negated. A comparison takes its opposite operator. A len,
// cap or copy count against zero takes ==, since a count is never negative. An
// expression already behind a ! loses it, and anything else gets one in front.
// An ordering over floats takes the ! around the whole expression instead,
// since NaN makes both halves false.
//
// Not every pair can be rewritten. An if with an init statement has nowhere to
// put the init in a return. A link in an else if chain cannot be replaced by a
// return. A condition that is itself a conjunction or a disjunction is left
// alone when its branch returns false first, because a ! around the whole
// expression reads worse than the if. A condition of another type cannot be
// returned in place of the result type. A condition with a line break inside
// it, such as a raw string or a multi-line literal body, has no one-line
// return to write. A pair with a comment inside it, or one whose
// return would run past the width limit that linelen configures, is reported
// without a fix: a comment inside the pair would be lost, and the wrap is the
// author's to place. A comment that trails the pair stays where it is, at the
// end of the return that replaces it.
//
// The rule is like staticcheck's S1008, which collapses the pair that ends a
// block; this also reports the pair under an else, and the pair below a guard.
package boolreturn

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "boolreturn",
	Doc:  "reports two returns of opposite boolean constants under an if",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var comments []*ast.Comment
		for _, group := range file.Comments {
			comments = append(comments, group.List...)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				scan(pass, comments, n.Body, n.Type)
				return false
			case *ast.FuncLit:
				scan(pass, comments, n.Body, n.Type)
				return false
			}
			return true
		})
	}
	return nil, nil
}

// scan checks one function body, and the literal bodies nested in it, whose
// result types are their own.
func scan(pass *analysis.Pass, comments []*ast.Comment, body *ast.BlockStmt, fn *ast.FuncType) {
	if body == nil {
		return
	}
	var (
		result = resultType(pass, fn)
		chain  = make(map[*ast.IfStmt]struct{})
	)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			scan(pass, comments, n.Body, n.Type)
			return false
		case *ast.BlockStmt:
			checkList(pass, comments, n.List, result)
		case *ast.CaseClause:
			checkList(pass, comments, n.Body, result)
		case *ast.CommClause:
			checkList(pass, comments, n.Body, result)
		case *ast.IfStmt:
			// An else if link stands where only a block or an if can, so a
			// return cannot take its place. Inspect reaches a parent first.
			_, chained := chain[n]
			if next, ok := n.Else.(*ast.IfStmt); ok {
				chain[next] = struct{}{}
			}
			if !chained {
				checkIf(pass, comments, n, result)
			}
		}
		return true
	})
}

// checkList reports the if and the return that end a list of statements when
// they return opposite constants.
func checkList(pass *analysis.Pass, comments []*ast.Comment, list []ast.Stmt, result types.Type) {
	n := len(list)
	if n < 2 {
		return
	}
	ifStmt, ok := list[n-2].(*ast.IfStmt)
	if !ok || ifStmt.Else != nil {
		return
	}
	tail, ok := list[n-1].(*ast.ReturnStmt)
	if !ok {
		return
	}
	ok, trueFirst := pair(pass, ifStmt.Body, tail)
	if !ok {
		return
	}
	report(pass, comments, ifStmt, tail.End(), trueFirst, result)
}

// checkIf reports an if whose else branch returns the other boolean constant.
func checkIf(pass *analysis.Pass, comments []*ast.Comment, ifStmt *ast.IfStmt, result types.Type) {
	elseBlock, ok := ifStmt.Else.(*ast.BlockStmt)
	if !ok {
		return
	}
	second, ok := sole(elseBlock)
	if !ok {
		return
	}
	ok, trueFirst := pair(pass, ifStmt.Body, second)
	if !ok {
		return
	}
	report(pass, comments, ifStmt, ifStmt.End(), trueFirst, result)
}

// pair reports whether two returns return opposite boolean constants, and
// whether the first of them returns true.
func pair(pass *analysis.Pass, block *ast.BlockStmt, second *ast.ReturnStmt) (ok, trueFirst bool) {
	first, ok := sole(block)
	if !ok {
		return false, false
	}
	trueFirst, ok = truth(pass, first)
	if !ok {
		return false, false
	}
	last, ok := truth(pass, second)
	return ok && trueFirst != last, trueFirst
}

// sole returns the return statement a block holds as its only statement.
func sole(block *ast.BlockStmt) (*ast.ReturnStmt, bool) {
	if len(block.List) != 1 {
		return nil, false
	}
	ret, ok := block.List[0].(*ast.ReturnStmt)
	return ret, ok
}

// truth returns the value of the boolean constant a return names as its only
// result.
func truth(pass *analysis.Pass, ret *ast.ReturnStmt) (value, ok bool) {
	if len(ret.Results) != 1 {
		return false, false
	}
	id, ok := ast.Unparen(ret.Results[0]).(*ast.Ident)
	if !ok {
		return false, false
	}
	obj := pass.TypesInfo.Uses[id]
	if obj == types.Universe.Lookup("true") {
		return true, true
	}
	if obj == types.Universe.Lookup("false") {
		return false, true
	}
	return false, false
}

// report reports the pair, which runs from the if to end, and suggests the one
// return that replaces it.
func report(pass *analysis.Pass, comments []*ast.Comment, ifStmt *ast.IfStmt, end token.Pos, trueFirst bool, result types.Type) {
	if ifStmt.Init != nil {
		return
	}
	cond := ast.Unparen(ifStmt.Cond)
	// The return has to type-check where the two constants did: a condition
	// of one type cannot be returned in place of a named result type.
	if !assignable(pass, cond, result) {
		return
	}
	if !trueFirst {
		negated, ok := negate(pass, cond)
		if !ok {
			return
		}
		cond = negated
	}
	source, ok := render(pass, cond)
	if !ok {
		return
	}
	text := append([]byte("return "), source...)
	diagnostic := analysis.Diagnostic{
		Pos:     ifStmt.Pos(),
		End:     end,
		Message: "two returns of opposite constants; write " + string(text),
	}
	edit, ok := replace(pass, comments, ifStmt, end, text)
	if !ok {
		return
	}
	// A line past the width limit is the author's to wrap, so it is reported
	// without a fix.
	if fits(pass, ifStmt, edit.NewText) {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Return the condition",
			TextEdits: []analysis.TextEdit{edit},
		}}
	}
	pass.Report(diagnostic)
}

// render returns cond as source on one line, and reports whether it has one.
// An empty FileSet has no file to map cond's positions to, so the printer lays
// the expression out itself and the line breaks the author used are dropped.
// Failing that, the author's own layout is tried, which keeps a literal body
// on one line. A condition with a line break inside it, such as a raw string,
// has no one-line form at all.
func render(pass *analysis.Pass, cond ast.Expr) ([]byte, bool) {
	for _, fset := range []*token.FileSet{token.NewFileSet(), pass.Fset} {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, cond); err != nil {
			return nil, false
		}
		if !bytes.ContainsRune(buf.Bytes(), '\n') {
			return buf.Bytes(), true
		}
	}
	return nil, false
}

// fits reports whether the last line of text stays within the width limit at
// the if's indent.
func fits(pass *analysis.Pass, ifStmt *ast.IfStmt, text []byte) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	file := pass.Fset.File(ifStmt.Pos())
	if file == nil {
		return false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	first := file.Offset(ifStmt.Pos())
	last := text[bytes.LastIndexByte(text, '\n')+1:]
	line := slices.Concat(src[lineStart(src, first):first], last)
	return linelen.CheckLine(line)
}

// replace returns the edit that puts text in place of the pair, carrying the
// comments inside it above the new line so none is lost or left describing a
// branch that is gone.
func replace(pass *analysis.Pass, comments []*ast.Comment, ifStmt *ast.IfStmt, end token.Pos, text []byte) (analysis.TextEdit, bool) {
	file := pass.Fset.File(ifStmt.Pos())
	if file == nil {
		return analysis.TextEdit{}, false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return analysis.TextEdit{}, false
	}
	first := file.Offset(ifStmt.Pos())
	stop := lineEnd(src, file.Offset(end))
	var lines [][]byte
	for _, comment := range comments {
		if off := file.Offset(comment.Pos()); off >= first && off < stop {
			lines = append(lines, []byte(comment.Text))
		}
	}
	lines = append(lines, text)
	indent := slices.Concat([]byte("\n"), src[lineStart(src, first):first])
	return analysis.TextEdit{
		Pos:     ifStmt.Pos(),
		End:     file.Pos(stop),
		NewText: bytes.Join(lines, indent),
	}, true
}

// lineStart returns the offset of the first byte of the line holding off.
func lineStart(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}

// lineEnd returns the offset of the newline that ends the line holding off.
func lineEnd(src []byte, off int) int {
	for off < len(src) && src[off] != '\n' {
		off++
	}
	return off
}

// negate returns the expression that holds where cond does not, and reports
// whether it has one. A ! around a conjunction or a disjunction reads worse
// than the if it would replace, so those have no negation.
func negate(pass *analysis.Pass, cond ast.Expr) (ast.Expr, bool) {
	switch cond := cond.(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.Uses[cond]
		if obj == types.Universe.Lookup("true") {
			return ast.NewIdent("false"), true
		}
		if obj == types.Universe.Lookup("false") {
			return ast.NewIdent("true"), true
		}
		return not(cond), true
	case *ast.UnaryExpr:
		if cond.Op == token.NOT {
			return ast.Unparen(cond.X), true
		}
		return not(cond), true
	case *ast.BinaryExpr:
		if eq, ok := countZero(pass, cond); ok {
			return eq, true
		}
		op, ok := flipped[cond.Op]
		if !ok {
			return nil, false
		}
		if !flippable(pass, cond) {
			return not(&ast.ParenExpr{X: cond}), true
		}
		return &ast.BinaryExpr{X: cond.X, Op: op, Y: cond.Y}, true
	case *ast.CallExpr, *ast.IndexExpr, *ast.SelectorExpr, *ast.StarExpr,
		*ast.TypeAssertExpr:
		return not(cond), true
	}
	return nil, false
}

// countZero returns count == 0 when bin tests a builtin count against zero for
// exceeding it, and reports whether bin is that shape.
func countZero(pass *analysis.Pass, bin *ast.BinaryExpr) (ast.Expr, bool) {
	count, zero := bin.X, bin.Y
	switch bin.Op {
	case token.GTR:
	case token.LSS:
		count, zero = bin.Y, bin.X
	default:
		return nil, false
	}
	call, ok := ast.Unparen(count).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	name, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil, false
	}
	builtin, ok := pass.TypesInfo.Uses[name].(*types.Builtin)
	if !ok {
		return nil, false
	}
	switch builtin.Name() {
	case "len", "cap", "copy":
	default:
		return nil, false
	}
	lit, ok := ast.Unparen(zero).(*ast.BasicLit)
	if !ok || lit.Kind != token.INT || lit.Value != "0" {
		return nil, false
	}
	return &ast.BinaryExpr{X: bin.X, Op: token.EQL, Y: bin.Y}, true
}

// not returns cond with a ! in front.
func not(cond ast.Expr) ast.Expr {
	return &ast.UnaryExpr{Op: token.NOT, X: cond}
}

// flippable reports whether a comparison's operators are exact complements. An
// equality's operators always are. An ordering's are not over floats, where
// NaN makes both x < 0 and x >= 0 false.
func flippable(pass *analysis.Pass, bin *ast.BinaryExpr) bool {
	switch bin.Op {
	case token.EQL, token.NEQ:
		return true
	}
	for _, operand := range [2]ast.Expr{bin.X, bin.Y} {
		typ := pass.TypesInfo.TypeOf(operand)
		if typ == nil {
			return false
		}
		basic, ok := typ.Underlying().(*types.Basic)
		if !ok || basic.Info()&(types.IsInteger|types.IsString) == 0 {
			return false
		}
	}
	return true
}

// flipped maps a comparison to the one that holds where it does not.
var flipped = map[token.Token]token.Token{
	token.EQL: token.NEQ,
	token.NEQ: token.EQL,
	token.LSS: token.GEQ,
	token.GEQ: token.LSS,
	token.GTR: token.LEQ,
	token.LEQ: token.GTR,
}

// resultType returns the type a function returns, and nil unless it returns
// exactly one value.
func resultType(pass *analysis.Pass, fn *ast.FuncType) types.Type {
	if fn.Results == nil || len(fn.Results.List) != 1 {
		return nil
	}
	if len(fn.Results.List[0].Names) > 1 {
		return nil
	}
	return pass.TypesInfo.TypeOf(fn.Results.List[0].Type)
}

// assignable reports whether the condition can be returned where the two
// constants were.
func assignable(pass *analysis.Pass, cond ast.Expr, result types.Type) bool {
	if result == nil {
		return false
	}
	condType := pass.TypesInfo.TypeOf(cond)
	return condType != nil && types.AssignableTo(condType, result)
}
