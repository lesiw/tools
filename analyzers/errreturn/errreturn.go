// Package errreturn provides an analysis.Analyzer that reports an if that
// returns the value it tests against nil, followed by a return of nil.
//
// The branch returns the value when it is not nil, and the trailing return
// yields nil otherwise, so return err covers both:
//
//	if err != nil {
//		return err
//	}
//	return nil
//
// With an init declaration the value is the declaration's expression, since
// the variable does not exist outside the if. The pair above becomes
// return load() when the if begins with err := load().
//
// The condition must compare a name with nil using !=, and the body must be
// one return of that same name. The value must have the function's result
// type. A concrete value returned as an interface is left alone, because a
// nil value of the concrete type becomes a non-nil interface, where the
// trailing return yields nil. The trailing return must be the next statement
// and must return nil. A different value, a comparison that uses ==, an extra
// statement, an else branch, and a result that is not nil are left alone. An
// if with an init is reported only when the init is a short declaration of
// one name from one expression, and that name must be the one the condition
// tests. The assignment form is left alone because the write may be
// observable outside the if, and more than one name has no single value to
// return.
//
// The fix replaces the if and the trailing return with the one return,
// written on one line. An init expression with a one-line form is collapsed
// onto that line. It is withheld, and the pair is still reported, when a
// comment inside the pair would be deleted, when the resulting line would be
// over the width limit that linelen configures, and when the init expression
// has no one-line form, as a raw string does.
//
// The rule is adapted from revive's if-return, which reports the form with an
// init declaration. The rule also reports the form without one.
package errreturn

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "errreturn",
	Doc:  "reports an if that returns the value it tests against nil, followed by a return of nil",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				scan(pass, file, n.Type, n.Body)
			case *ast.FuncLit:
				scan(pass, file, n.Type, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// scan checks the statement lists in one function body.
func scan(pass *analysis.Pass, file *ast.File, fn *ast.FuncType, body *ast.BlockStmt) {
	if body == nil {
		return
	}
	result := resultType(pass, fn)
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BlockStmt:
			checkList(pass, file, n.List, result)
		case *ast.CaseClause:
			checkList(pass, file, n.Body, result)
		case *ast.CommClause:
			checkList(pass, file, n.Body, result)
		}
		return true
	})
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

// checkList checks each adjacent pair in list.
func checkList(pass *analysis.Pass, file *ast.File, list []ast.Stmt, result types.Type) {
	for i := 0; i+1 < len(list); i++ {
		ifStmt, ok := list[i].(*ast.IfStmt)
		if !ok || ifStmt.Else != nil {
			continue
		}
		trailing, ok := list[i+1].(*ast.ReturnStmt)
		if !ok {
			continue
		}
		check(pass, file, ifStmt, trailing, result)
	}
}

// check reports the pair that ifStmt and trailing make when the if returns
// the value it tests and trailing returns nil.
func check(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, trailing *ast.ReturnStmt, result types.Type) {
	if !returnsNil(pass, trailing) {
		return
	}
	tested, ok := nilTest(pass, ifStmt.Cond)
	if !ok {
		return
	}
	returned, ok := soleReturn(ifStmt.Body)
	if !ok || !sameObject(pass, returned, tested) {
		return
	}
	expr, ok := value(pass, ifStmt, tested)
	if !ok || !sameType(pass, expr, result) {
		return
	}
	text, ok := replacement(pass, expr)
	message := "redundant if"
	if ok {
		message += "; write " + string(text)
	}
	diagnostic := analysis.Diagnostic{
		Pos:     ifStmt.Pos(),
		End:     trailing.End(),
		Message: message,
	}
	fixable := ok && !commented(file, ifStmt.Pos(), trailing.End()) &&
		fits(pass, ifStmt, trailing, text)
	if fixable {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Return the value",
			TextEdits: []analysis.TextEdit{{
				Pos:     ifStmt.Pos(),
				End:     trailing.End(),
				NewText: text,
			}},
		}}
	}
	pass.Report(diagnostic)
}

// sameType reports whether expr has the function's result type. A concrete
// value returned as an interface is left alone: a nil value of the concrete
// type becomes a non-nil interface, where the trailing return yields nil.
func sameType(pass *analysis.Pass, expr ast.Expr, result types.Type) bool {
	if result == nil {
		return false
	}
	typ := pass.TypesInfo.TypeOf(expr)
	if typ == nil {
		return false
	}
	return types.Identical(types.Unalias(typ), types.Unalias(result))
}

// nilTest returns the name a condition compares with nil using !=.
func nilTest(pass *analysis.Pass, cond ast.Expr) (*ast.Ident, bool) {
	bin, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return nil, false
	}
	if id, ok := ast.Unparen(bin.X).(*ast.Ident); ok && isNil(pass, bin.Y) {
		return id, true
	}
	if id, ok := ast.Unparen(bin.Y).(*ast.Ident); ok && isNil(pass, bin.X) {
		return id, true
	}
	return nil, false
}

// returnsNil reports whether ret returns nil as its only result.
func returnsNil(pass *analysis.Pass, ret *ast.ReturnStmt) bool {
	return len(ret.Results) == 1 && isNil(pass, ret.Results[0])
}

// isNil reports whether e is the predeclared nil.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == types.Universe.Lookup("nil")
}

// soleReturn returns the name a body holds as its only statement when that
// statement is a return of one name.
func soleReturn(body *ast.BlockStmt) (*ast.Ident, bool) {
	if body == nil || len(body.List) != 1 {
		return nil, false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, false
	}
	id, ok := ast.Unparen(ret.Results[0]).(*ast.Ident)
	return id, ok
}

// value returns the expression the replacement returns. Without an init it is
// the tested name; with one it is the expression the init declares. The
// declared name must be the tested name, since the replacement removes the
// declaration and cannot keep the write. A driver can analyze packages with
// type errors, so the name is checked rather than assumed.
func value(pass *analysis.Pass, ifStmt *ast.IfStmt, tested *ast.Ident) (ast.Expr, bool) {
	if ifStmt.Init == nil {
		return tested, true
	}
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || !sameObject(pass, lhs, tested) {
		return nil, false
	}
	return assign.Rhs[0], true
}

// sameObject reports whether a and b name the same object.
func sameObject(pass *analysis.Pass, a, b *ast.Ident) bool {
	obj := pass.TypesInfo.ObjectOf(a)
	return obj != nil && obj == pass.TypesInfo.ObjectOf(b)
}

// replacement returns the return statement that replaces the pair. An empty
// FileSet has no file to map the expression's positions to, so the expression
// is written without the author's line breaks. A form that still spans lines,
// such as a one-line function literal written over several lines, is tried
// with the pass's FileSet, which uses the author's positions. A form that
// still spans lines is tried with every position mapped to one line, so a
// function literal body is written on one line. A raw string has newlines in
// its value, so it has no one-line form.
func replacement(pass *analysis.Pass, expr ast.Expr) ([]byte, bool) {
	fsets := []*token.FileSet{
		token.NewFileSet(),
		pass.Fset,
		oneLine(pass, expr),
	}
	for _, fset := range fsets {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, expr); err != nil {
			return nil, false
		}
		if !bytes.ContainsRune(buf.Bytes(), '\n') {
			return append([]byte("return "), buf.Bytes()...), true
		}
	}
	return nil, false
}

// oneLine returns a FileSet whose file has no line table, so every position in
// the expression maps to one line.
func oneLine(pass *analysis.Pass, expr ast.Expr) *token.FileSet {
	var (
		fset = token.NewFileSet()
		file = pass.Fset.File(expr.Pos())
	)
	if file == nil {
		fset.AddFile("", -1, 1<<30)
	} else {
		fset.AddFile("", file.Base(), file.Size())
	}
	return fset
}

// commented reports whether a comment overlaps the text from pos to end.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && pos < group.End() {
			return true
		}
	}
	return false
}

// fits reports whether the replacement stays within the width limit on the
// line the if starts, including what follows the trailing return.
func fits(pass *analysis.Pass, ifStmt *ast.IfStmt, trailing *ast.ReturnStmt, text []byte) bool {
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
	start := file.Offset(ifStmt.Pos())
	end := file.Offset(trailing.End())
	if start < 0 || start > len(src) || end < start || end > len(src) {
		return false
	}
	indent := start
	for indent > 0 && src[indent-1] != '\n' {
		indent--
	}
	stop := end
	for stop < len(src) && src[stop] != '\n' {
		stop++
	}
	line := make([]byte, 0, start-indent+len(text)+stop-end)
	line = append(line, src[indent:start]...)
	line = append(line, text...)
	line = append(line, src[end:stop]...)
	return linelen.CheckLine(line)
}
