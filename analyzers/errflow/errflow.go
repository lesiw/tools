// Package errflow provides an analysis.Analyzer that reports a function which
// writes the same error path two ways.
//
// A step is one call and the guard that checks its error. Go keeps the normal
// path at the function's own level and indents the error handling, so a
// function that guards one step guards the next one the same way. Three shapes
// break that.
//
// An else after a block that ends in return, break, continue or goto runs only
// when the block did not, so its code could sit at the function's own level
// and the indent adds nothing:
//
//	if err != nil {
//		return 0
//	} else {
//		return 1
//	}
//
// The else rule reports only; ifreturn carries its fix for the body that only
// returns. An else-if chain and an if with an init statement are left alone.
//
// A guard that returns the value it tested, followed by a return of nil, is
// one return written twice:
//
//	if err := load(); err != nil {
//		return err
//	}
//	return nil
//
// The pair collapses into return load(), and the fix returns the expression
// the if's init statement declares. A step written on two statements collapses
// the same way: the statement that produces the value goes with the guard, and
// the return replaces both.
//
// A var block holding more than one spec is not the step, because replacing
// the block would delete its other specs. The block stays and the return names
// the value.
//
// The collapse leaves alone a guard whose condition uses ==, whose body
// returns a different value or holds more than one statement, or that has an
// else branch. It also leaves alone a declaration of more than one name or
// value, a value assigned rather than declared, and a value an earlier
// statement declares or assigns, unless that statement is a var block of more
// than one spec. A concrete value returned as an error is left alone too,
// because a nil value of the concrete type becomes a non-nil error.
//
// Where the guard is followed by a call instead of a return of nil, the call
// takes no guard:
//
//	if err := load(); err != nil {
//		return err
//	}
//	return save()
//
// The fix writes the guard the step above uses around the call, then returns
// nil.
//
// One list takes at most one of the two shapes, so no function reaches both
// fixes. The expansion adds a guard, which leaves the list with two nil checks
// and the collapse with a condition it does not meet. The collapse removes the
// list's only nil check, which leaves the expansion no guard to copy.
//
// Both shapes apply only when the function returns one result, and that result
// is error. A (T, error) result is left alone, because the guard would need a
// zero value for T.
//
// The collapse needs three things: the guard's condition compares a name with
// nil using !=, its body is one return of that same name, and the guard is the
// list's only nil check. The return writes the expression the value came from
// when the statement just above declares it, and names the value otherwise.
//
// The expansion needs a guard above the last statement that returns the value
// it tests, and it takes the name that guard tests. A last statement that is
// not a call, a call that returns another type, a conversion to error, and a
// call to errors.New or fmt.Errorf are left alone: a conversion is not a call,
// and the guard written around those two constructors would always take its
// branch. A constructor the rule cannot name is reported, since it may return
// nil, and an author whose own constructor never does writes //ignore:errflow.
// The check reads the function's own body, so a guard and a call inside a
// nested block are left alone.
//
// The collapse writes the expression as the author wrote it. It writes a
// comment between the guard's condition and its closing brace above the
// return, at the return's indent, and leaves a comment inside the expression
// where it is. The fix is withheld when it would delete any other comment, and
// when the nil return shares its line with one.
//
// The expansion writes a comment above the guard when the fix would delete it,
// and leaves one inside the call where it is. A comment after the return is
// written there when the return's line holds nothing after it but that comment
// and no comment begins on the next line: text after the comment would be left
// behind, and a comment below it would be separated from it.
//
// Either fix is withheld when a comment it moves is a directive, because a
// tool reads a directive where it is written, and when a comment it moves
// begins mid-line and covers more than one line, because the fix cannot
// reindent it. A directive inside the expression or the call is kept, because
// the fix writes that text as the author wrote it. Both shapes report even
// when the fix is withheld.
//
// The expansion reads the tab run that begins the line it replaces and writes
// one tab past it for the guard body, and it is withheld when the line begins
// with anything else. The collapse reads that run only to write a comment
// above the return, so a line indented another way still collapses when it
// holds no comment.
//
// The expansion puts the call on the header line, so a long call may make a
// line that linelen reports, and a call written across lines makes a header
// that linewrap reports. Hoist the call into a variable above the guard, and
// wrap its arguments there. The collapse writes return where the statement
// that produced the value began, so its first line is at most two columns
// wider than the one it replaces, and a line near linelen's limit may go past
// it.
//
// The else rule is inspired by revive's indent-error-flow. The guard that
// returns the value it tested is adapted from revive's if-return.
//
// The README catalogs every check, exemption, and fix.
package errflow

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/source"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "errflow",
	Doc:  "reports confusing or inconsistent error flows",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.IfStmt:
				checkElse(pass, n)
			case *ast.FuncDecl:
				checkPath(pass, file, n.Type, n.Body)
			case *ast.FuncLit:
				checkPath(pass, file, n.Type, n.Body)
			}
			return true
		})
	}
	return nil, nil
}

// checkElse reports an else block whose if block terminates.
func checkElse(pass *analysis.Pass, ifStmt *ast.IfStmt) {
	if ifStmt.Else == nil {
		return
	}
	// An else if is a chain, and a chain reads better than a
	// column of outdented ifs.
	if _, ok := ifStmt.Else.(*ast.IfStmt); ok {
		return
	}
	// With an init statement the else block can name values the
	// outdented form would not.
	if ifStmt.Init != nil {
		return
	}
	if !terminates(pass, ifStmt.Body) {
		return
	}
	pass.Reportf(ifStmt.Else.Pos(), "unnecessary else")
}

// terminates reports whether control cannot reach the end of block.
func terminates(pass *analysis.Pass, block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	switch stmt := block.List[len(block.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok != token.FALLTHROUGH
	case *ast.ExprStmt:
		return isPanic(pass, stmt.X)
	case *ast.IfStmt:
		// An if that terminates both ways terminates the block.
		return stmt.Else != nil && terminates(pass, stmt.Body) &&
			terminates(pass, asBlock(stmt.Else))
	}
	return false
}

// asBlock returns the body of an else statement, if it has one.
func asBlock(stmt ast.Stmt) *ast.BlockStmt {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		return stmt
	case *ast.IfStmt:
		return &ast.BlockStmt{List: []ast.Stmt{stmt}}
	}
	return nil
}

// isPanic reports whether e is a call to the builtin panic.
func isPanic(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	return pass.TypesInfo.Uses[id] == types.Universe.Lookup("panic")
}

// checkPath checks one function: the trailing pair of every statement list in
// it, and the last step of its body.
func checkPath(pass *analysis.Pass, file *ast.File, fn *ast.FuncType, body *ast.BlockStmt) {
	if body == nil {
		return
	}
	result := soleError(pass, fn)
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
	if result {
		expand(pass, file, body.List)
	}
}

// checkList reports a list that ends in a nil guard and a return of nil, when
// the guard is the list's only nil check and step finds where the value it
// tests comes from.
func checkList(pass *analysis.Pass, file *ast.File, list []ast.Stmt, errorResult bool) {
	if !errorResult || len(list) < 2 {
		return
	}
	var (
		last    = len(list) - 1
		ret, ok = list[last].(*ast.ReturnStmt)
	)
	if !ok || !returnsNil(pass, ret) {
		return
	}
	guard, ok := list[last-1].(*ast.IfStmt)
	if !ok || guard.Else != nil || nilChecks(pass, list) != 1 {
		return
	}
	tested, ok := nilTest(pass, guard.Cond)
	if !ok {
		return
	}
	returned, ok := soleReturn(guard.Body)
	if !ok || !sameObject(pass, returned, tested) {
		return
	}
	expr, start, ok := step(pass, list[:last-1], guard, tested)
	if !ok || !isError(pass, expr) {
		return
	}
	collapse(pass, file, trailing{
		start: start,
		expr:  expr,
		guard: guard,
		ret:   ret,
	})
}

// soleError reports whether fn's results are one error.
func soleError(pass *analysis.Pass, fn *ast.FuncType) bool {
	if fn.Results == nil || len(fn.Results.List) != 1 {
		return false
	}
	field := fn.Results.List[0]
	if len(field.Names) > 1 {
		return false
	}
	return isError(pass, field.Type)
}

// trailing is the span a collapse replaces, from the first statement the edit
// covers to the return of nil that closes the list, with the guard that tested
// the value and the value itself.
type trailing struct {
	start ast.Stmt
	expr  ast.Expr
	guard *ast.IfStmt
	ret   *ast.ReturnStmt
}

// collapse reports a guard that only returns the value it tests, where the nil
// return that follows makes the guard redundant.
func collapse(pass *analysis.Pass, file *ast.File, tail trailing) {
	text, ok := textOf(pass, tail.expr)
	if !ok {
		return
	}
	fix := place(pass, file, tail, append([]byte("return "), text...))
	// The message names the text the fix writes, so a reader who runs the
	// command without -fix sees the same statement. A withheld fix names
	// nothing, and so does one that writes more than one line.
	message := "redundant guard"
	if fix != nil && !bytes.ContainsRune(fix, '\n') {
		message += "; write " + string(fix)
	}
	diagnostic := analysis.Diagnostic{
		Pos:     tail.start.Pos(),
		End:     tail.ret.End(),
		Message: message,
	}
	if fix != nil {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Return the value",
			TextEdits: []analysis.TextEdit{{
				Pos:     tail.start.Pos(),
				End:     tail.ret.End(),
				NewText: fix,
			}},
		}}
	}
	pass.Report(diagnostic)
}

// place returns the replacement text, with a comment between the guard's
// condition and its closing brace written above the return. It returns nil
// when a comment the fix would delete has no place there.
func place(pass *analysis.Pass, file *ast.File, tail trailing, text []byte) []byte {
	var (
		stop  = lineEnd(pass, tail.ret.End())
		moved = between(file, tail.start.Pos(), stop, tail.expr)
	)
	if len(moved) == 0 {
		return text
	}
	indent, ok := indentOf(pass, tail.start.Pos())
	if !ok {
		return nil
	}
	lead := make([]string, 0, len(moved))
	var prev token.Pos
	for _, group := range moved {
		inside := tail.guard.Cond.End() <= group.Pos() &&
			group.Pos() < tail.guard.End()
		if !inside || directive(group) {
			return nil
		}
		lines, ok := groupText(pass, group, indent)
		if !ok {
			return nil
		}
		lead = append(lead, gap(pass, prev, group, indent)+lines)
		prev = group.End()
	}
	head := strings.Join(lead, "") + "\n" + indent
	return append([]byte(head), text...)
}

// gap returns what separates prev from group in the text the fix writes: an
// empty string when group is the first, a line break and the new indent when
// the author wrote them on one line or on consecutive lines, and a blank line
// and the new indent when the author left one.
func gap(pass *analysis.Pass, prev token.Pos, group *ast.CommentGroup, indent string) string {
	if !prev.IsValid() {
		return ""
	}
	prevLine := pass.Fset.Position(prev).Line
	if next := pass.Fset.Position(group.Pos()).Line; next <= prevLine+1 {
		return "\n" + indent
	}
	return "\n\n" + indent
}

// expand reports a body that ends in a call that takes no guard.
func expand(pass *analysis.Pass, file *ast.File, list []ast.Stmt) {
	if len(list) < 2 {
		return
	}
	ret, ok := list[len(list)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return
	}
	call, ok := ast.Unparen(ret.Results[0]).(*ast.CallExpr)
	if !ok || !isError(pass, call) {
		return
	}
	if isConversion(pass, call) || constructs(pass, call) {
		return
	}
	name, ok := guardName(pass, list[:len(list)-1])
	if !ok {
		return
	}
	// The message names the guard the fix writes, so a reader who runs the
	// command without -fix sees what the check is looking for. A withheld fix
	// names nothing, and so does a call written across lines.
	message := "unguarded last call"
	if text, ok := textOf(pass, call); ok && !bytes.ContainsRune(text, '\n') {
		message += fmt.Sprintf(
			"; write if %s := %s; %s != nil { return %s }",
			name, text, name, name,
		)
	}
	diagnostic := analysis.Diagnostic{
		Pos:     ret.Pos(),
		End:     ret.End(),
		Message: message,
	}
	if edit, ok := guardEdit(pass, file, ret, call, name); ok {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Guard the call",
			TextEdits: []analysis.TextEdit{edit},
		}}
	}
	pass.Report(diagnostic)
}

// guardEdit returns the edit that writes the guard around the call, and
// reports false when no edit can be written.
func guardEdit(pass *analysis.Pass, file *ast.File, ret *ast.ReturnStmt, call *ast.CallExpr, name string) (analysis.TextEdit, bool) {
	indent, ok := indentOf(pass, ret.Pos())
	if !ok {
		return analysis.TextEdit{}, false
	}
	head, end, ok := displaced(pass, file, ret, call, indent)
	if !ok {
		return analysis.TextEdit{}, false
	}
	text, ok := textOf(pass, call)
	if !ok {
		return analysis.TextEdit{}, false
	}
	var expansion bytes.Buffer
	if head != "" {
		expansion.WriteString(head)
		expansion.WriteString("\n")
		expansion.WriteString(indent)
	}
	fmt.Fprintf(&expansion, "if %s := %s; %s != nil {\n", name, text, name)
	fmt.Fprintf(&expansion, "%s\treturn %s\n", indent, name)
	fmt.Fprintf(&expansion, "%s}\n", indent)
	fmt.Fprintf(&expansion, "%sreturn nil", indent)
	return analysis.TextEdit{
		Pos:     ret.Pos(),
		End:     end,
		NewText: expansion.Bytes(),
	}, true
}

// displaced returns the comments the expansion would delete, as the text to
// write above the guard, and the position the edit must end at, which covers
// the last comment on the line and is never before the return's end. A comment
// inside the call stays where it is, because the fix writes the call's text as
// the author wrote it. A comment before the call is outside that text, and so
// is one after the return; the fix writes both above the guard when trails
// allows it, and is withheld when it does not.
func displaced(pass *analysis.Pass, file *ast.File, ret *ast.ReturnStmt, call *ast.CallExpr, indent string) (string, token.Pos, bool) {
	var (
		end   = ret.End()
		stop  = lineEnd(pass, ret.End())
		parts []string
	)
	for _, group := range between(file, ret.Pos(), stop, call) {
		if directive(group) || !trails(pass, file, ret, group) {
			return "", end, false
		}
		text, ok := groupText(pass, group, indent)
		if !ok {
			return "", end, false
		}
		parts = append(parts, text)
		if group.End() > end {
			end = group.End()
		}
	}
	return strings.Join(parts, "\n"+indent), end, true
}

// trails reports whether group can be written above the guard. A group the
// return holds is deleted with the return, so the rest of the line has nothing
// to leave behind. A group after the return needs a line that holds nothing
// else after group, since that text would be left behind, and no comment
// beginning on the next line, since the two read as one comment and the fix
// would move only the first.
func trails(pass *analysis.Pass, file *ast.File, ret *ast.ReturnStmt, group *ast.CommentGroup) bool {
	if group.Pos() < ret.End() {
		return true
	}
	tokFile, src, ok := readSource(pass, ret.Pos())
	if !ok {
		return false
	}
	lo := tokFile.Offset(ret.End())
	hi := tokFile.Offset(group.Pos())
	end := tokFile.Offset(group.End())
	if lo < 0 || lo > hi || end < hi || end > len(src) {
		return false
	}
	line := source.End(src, lo)
	if line > 0 && line <= len(src) && src[line-1] == '\n' {
		line--
	}
	if line < end || !source.Blank(src[end:line]) {
		return false
	}
	if strings.Trim(string(src[lo:hi]), " \t;") != "" {
		return false
	}
	next := source.End(src, source.End(src, line))
	return next <= tokFile.Size() &&
		len(between(file, group.End(), tokFile.Pos(next), nil)) == 0
}

// isConversion reports whether call converts a value, which a call expression
// writes but a step cannot be.
func isConversion(pass *analysis.Pass, call *ast.CallExpr) bool {
	tv, ok := pass.TypesInfo.Types[call.Fun]
	return ok && tv.IsType()
}

// constructs reports whether call builds an error. errors.New and fmt.Errorf
// always return a non-nil error, so the guard the expansion writes would
// always take its branch.
func constructs(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil || fn.Signature().Recv() != nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "errors":
		return fn.Name() == "New"
	case "fmt":
		return fn.Name() == "Errorf"
	}
	return false
}

// guardName returns the name the last guard above the tail tests.
func guardName(pass *analysis.Pass, head []ast.Stmt) (string, bool) {
	for _, h := range slices.Backward(head) {
		ifStmt, ok := h.(*ast.IfStmt)
		if !ok || ifStmt.Else != nil {
			continue
		}
		tested, ok := nilTest(pass, ifStmt.Cond)
		if !ok {
			continue
		}
		returned, ok := soleReturn(ifStmt.Body)
		if !ok || !sameObject(pass, returned, tested) {
			continue
		}
		return tested.Name, true
	}
	return "", false
}

// readSource returns the file pos is written in, with its text.
func readSource(pass *analysis.Pass, pos token.Pos) (*token.File, []byte, bool) {
	file := pass.Fset.File(pos)
	if file == nil {
		return nil, nil, false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return nil, nil, false
	}
	return file, src, true
}

// textOf returns the text of node as the author wrote it.
func textOf(pass *analysis.Pass, node ast.Node) ([]byte, bool) {
	file, src, ok := readSource(pass, node.Pos())
	if !ok {
		return nil, false
	}
	start, end := file.Offset(node.Pos()), file.Offset(node.End())
	if start < 0 || end < start || end > len(src) {
		return nil, false
	}
	return src[start:end], true
}

// indentOf returns the tabs between pos and the start of its line, and false
// when the line holds anything else there.
func indentOf(pass *analysis.Pass, pos token.Pos) (string, bool) {
	file, src, ok := readSource(pass, pos)
	if !ok {
		return "", false
	}
	off := file.Offset(pos)
	if off < 0 || off > len(src) {
		return "", false
	}
	start := source.Start(src, off)
	for _, b := range src[start:off] {
		if b != '\t' {
			return "", false
		}
	}
	return string(src[start:off]), true
}

// lineEnd returns the position of the newline that ends p's line, or the
// position of the line's last byte when the file does not end it with one. It
// reads the bytes rather than the line table, because File.Line renumbers the
// lines after a line directive and LineStart does not, so the two count from
// different places.
func lineEnd(pass *analysis.Pass, p token.Pos) token.Pos {
	file, src, ok := readSource(pass, p)
	if !ok {
		return p
	}
	off := file.Offset(p)
	if off < 0 || off > len(src) {
		return p
	}
	if end := source.End(src, off); end > off && end <= file.Size() {
		return file.Pos(end - 1)
	}
	return p
}

// isError reports whether e has type error.
func isError(pass *analysis.Pass, e ast.Expr) bool {
	typ := pass.TypesInfo.TypeOf(e)
	if typ == nil {
		return false
	}
	errorType := types.Universe.Lookup("error").Type()
	return types.Identical(types.Unalias(typ), errorType)
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

// step returns the expression the tested value comes from and the statement
// that produces it. The guard's own init declares it, or the statement just
// above declares it from one expression. A value declared any other way, or
// assigned to, has no expression the collapse could return in its place.
func step(pass *analysis.Pass, above []ast.Stmt, guard *ast.IfStmt, tested *ast.Ident) (ast.Expr, ast.Stmt, bool) {
	if guard.Init != nil {
		assign, ok := guard.Init.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE {
			return nil, nil, false
		}
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return nil, nil, false
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || !sameObject(pass, lhs, tested) {
			return nil, nil, false
		}
		return assign.Rhs[0], guard, true
	}
	if len(above) > 0 {
		prev := above[len(above)-1]
		if expr, declared := declares(pass, prev, tested); declared {
			if expr == nil {
				return nil, nil, false
			}
			return expr, prev, true
		}
		if assigns(pass, prev, tested) {
			return nil, nil, false
		}
		// A value declared further up the list is not the step the guard
		// reads, and the collapse would leave the declaration behind.
		for _, stmt := range above[:len(above)-1] {
			if _, declared := declares(pass, stmt, tested); declared {
				return nil, nil, false
			}
			if assigns(pass, stmt, tested) {
				return nil, nil, false
			}
		}
	}
	return tested, guard, true
}

// declares returns the expression a statement declares tested from. It reports
// a declaration of tested even when it names no such single expression, as a
// two-name short declaration does. A var block holding more than one spec is
// not such a declaration: replacing the block would delete its other specs, so
// the collapse names the value instead.
func declares(pass *analysis.Pass, stmt ast.Stmt, tested *ast.Ident) (ast.Expr, bool) {
	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		if stmt.Tok == token.DEFINE && names(pass, stmt.Lhs, tested) {
			return single(stmt.Rhs, len(stmt.Lhs))
		}
	case *ast.DeclStmt:
		decl, ok := stmt.Decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			return nil, false
		}
		if len(decl.Specs) != 1 {
			return nil, false
		}
		spec, ok := decl.Specs[0].(*ast.ValueSpec)
		if !ok || !objects(pass, spec.Names, tested) {
			return nil, false
		}
		return single(spec.Values, len(spec.Names))
	}
	return nil, false
}

// names reports whether exprs name tested.
func names(pass *analysis.Pass, exprs []ast.Expr, tested *ast.Ident) bool {
	for _, expr := range exprs {
		id, ok := ast.Unparen(expr).(*ast.Ident)
		if ok && sameObject(pass, id, tested) {
			return true
		}
	}
	return false
}

// objects reports whether names name tested.
func objects(pass *analysis.Pass, names []*ast.Ident, tested *ast.Ident) bool {
	for _, name := range names {
		if sameObject(pass, name, tested) {
			return true
		}
	}
	return false
}

// single returns the one value a declaration of count names holds, and reports
// a declaration of the right shape even when it names no single value.
func single(values []ast.Expr, count int) (ast.Expr, bool) {
	if count != 1 || len(values) != 1 {
		return nil, true
	}
	return values[0], true
}

// assigns reports whether stmt writes tested without declaring it.
func assigns(pass *analysis.Pass, stmt ast.Stmt, tested *ast.Ident) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return false
	}
	return names(pass, assign.Lhs, tested)
}

// nilChecks returns the number of statements in list that compare a value with
// nil. A list with more than one is a body of two steps, which the expansion
// guards, so the collapse leaves it alone.
func nilChecks(pass *analysis.Pass, list []ast.Stmt) (n int) {
	for _, stmt := range list {
		ifStmt, ok := stmt.(*ast.IfStmt)
		if ok && nilCompared(pass, ifStmt.Cond) {
			n++
		}
	}
	return n
}

// nilCompared reports whether cond compares a value with nil.
func nilCompared(pass *analysis.Pass, cond ast.Expr) bool {
	bin, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok || (bin.Op != token.EQL && bin.Op != token.NEQ) {
		return false
	}
	return isNil(pass, bin.X) || isNil(pass, bin.Y)
}

// sameObject reports whether a and b name the same object.
func sameObject(pass *analysis.Pass, a, b *ast.Ident) bool {
	obj := pass.TypesInfo.ObjectOf(a)
	return obj != nil && obj == pass.TypesInfo.ObjectOf(b)
}

// groupText returns group written at indent. The indentation the group began
// with is dropped from every line after the first, so a block comment keeps
// its shape under the new indent. An empty line is written empty, so the fix
// leaves no trailing whitespace.
func groupText(pass *analysis.Pass, group *ast.CommentGroup, indent string) (string, bool) {
	src, ok := textOf(pass, group)
	if !ok {
		return "", false
	}
	lines := strings.Split(string(src), "\n")
	if len(lines) == 1 {
		return string(src), true
	}
	base, ok := indentOf(pass, group.Pos())
	if !ok {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimPrefix(lines[i], base)
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n"), true
}

// between returns the comment groups that begin in lo..hi, leaving out a group
// inside n, whose text the fix already writes.
func between(file *ast.File, lo, hi token.Pos, n ast.Node) (out []*ast.CommentGroup) {
	for _, group := range file.Comments {
		if group.Pos() < lo || hi <= group.Pos() {
			continue
		}
		if n != nil && n.Pos() <= group.Pos() && group.End() <= n.End() {
			continue
		}
		out = append(out, group)
	}
	return out
}

// directive reports whether group holds a comment a tool reads at its
// position: a line comment with no space after its slashes, and the block form
// of a line directive. A list of names would miss a tool the suite does not
// know; a comment with no space is not prose to this suite, whatever reads it.
func directive(group *ast.CommentGroup) bool {
	for _, comment := range group.List {
		text := comment.Text
		if strings.HasPrefix(text, "/*line ") {
			return true
		}
		if strings.HasPrefix(text, "//") && len(text) > 2 && text[2] != ' ' {
			return true
		}
	}
	return false
}
