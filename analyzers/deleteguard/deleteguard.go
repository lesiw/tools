// Package deleteguard provides an analysis.Analyzer that reports an existence
// check around a delete of the same key.
//
// The documentation for delete says "If m is nil or there is no such element,
// delete is a no-op." The check therefore does not affect the delete.
//
// The fix replaces the if with the delete in its body, or with a block around
// that delete when the if is an else-if with no else. It is withheld where a
// comment inside the if, on the if's first or closing line, or between a
// branch and an else-if would be lost or left describing the check, where the
// delete has no one-line form, or where the line it writes would run past the
// length limit while the guard's first line stays within it.
//
// The rule is adapted from staticcheck's S1033, which reports the same guard.
package deleteguard

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "deleteguard",
	Doc:  "reports an existence check around a delete of the same key",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		chained := elseIfs(file)
		ast.Inspect(file, func(n ast.Node) bool {
			if ifStmt, ok := n.(*ast.IfStmt); ok {
				examine(pass, file, ifStmt, chained[ifStmt])
			}
			return true
		})
	}
	return nil, nil
}

// elseIfs returns the position after the closing brace that precedes each
// chained if. An else accepts only an if or a block, so the delete needs a
// block.
func elseIfs(file *ast.File) map[*ast.IfStmt]token.Pos {
	chained := make(map[*ast.IfStmt]token.Pos)
	ast.Inspect(file, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			markElseIfs(ifStmt, chained)
		}
		return true
	})
	return chained
}

// markElseIfs records the position after the preceding body's closing brace
// for each if in the chain.
func markElseIfs(ifStmt *ast.IfStmt, chained map[*ast.IfStmt]token.Pos) {
	for {
		next, ok := ifStmt.Else.(*ast.IfStmt)
		if !ok {
			return
		}
		chained[next] = ifStmt.Body.Rbrace + 1
		ifStmt = next
	}
}

// examine reports ifStmt when its condition tests whether the key is present
// and its only statement deletes that key.
func examine(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, elsePos token.Pos) {
	call, ok := guardedDelete(pass, ifStmt)
	if !ok {
		return
	}
	d := analysis.Diagnostic{
		Pos:     ifStmt.Pos(),
		End:     ifStmt.End(),
		Message: "delete does not need a presence check",
	}
	if edits, ok := fix(pass, file, ifStmt, call, elsePos); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the check",
			TextEdits: edits,
		}}
	}
	pass.Report(d)
}

// guardedDelete returns the delete call in ifStmt's body when the if tests the
// presence of the key that call deletes, and reports false for any other if.
func guardedDelete(pass *analysis.Pass, ifStmt *ast.IfStmt) (*ast.CallExpr, bool) {
	if ifStmt.Else != nil || ifStmt.Init == nil {
		return nil, false
	}
	index, ok := presenceCheck(pass, ifStmt)
	if !ok {
		return nil, false
	}
	call, ok := deleteBody(pass, ifStmt.Body)
	if !ok {
		return nil, false
	}
	return call, astexpr.Same(pass, call.Args[0], index.X) &&
		astexpr.Same(pass, call.Args[1], index.Index)
}

// presenceCheck returns the map index whose presence ifStmt tests, and reports
// false for any other condition.
func presenceCheck(pass *analysis.Pass, ifStmt *ast.IfStmt) (*ast.IndexExpr, bool) {
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return nil, false
	}
	if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
		return nil, false
	}
	blank, ok := ast.Unparen(assign.Lhs[0]).(*ast.Ident)
	if !ok || blank.Name != "_" {
		return nil, false
	}
	flag, ok := ast.Unparen(assign.Lhs[1]).(*ast.Ident)
	if !ok || flag.Name == "_" {
		return nil, false
	}
	index, ok := ast.Unparen(assign.Rhs[0]).(*ast.IndexExpr)
	if !ok || !astexpr.Same(pass, ifStmt.Cond, flag) {
		return nil, false
	}
	return index, true
}

// deleteBody returns the delete call that is the only statement in body, and
// reports false for any other body.
func deleteBody(pass *analysis.Pass, body *ast.BlockStmt) (*ast.CallExpr, bool) {
	if len(body.List) != 1 {
		return nil, false
	}
	stmt, ok := body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return nil, false
	}
	name, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[name] != types.Universe.Lookup("delete") {
		return nil, false
	}
	return call, true
}

// fix returns the edits that write the delete in place of the if, and reports
// false when the edit would delete a comment or leave it describing the check,
// write a line past the width, or write a call with no one-line form.
func fix(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, call *ast.CallExpr, elsePos token.Pos) ([]analysis.TextEdit, bool) {
	if commented(pass, file, ifStmt, elsePos) {
		return nil, false
	}
	text, ok := astexpr.OneLine(call)
	if !ok {
		return nil, false
	}
	chained := elsePos.IsValid()
	if !fits(pass, ifStmt, text, chained) {
		return nil, false
	}
	if !chained {
		return []analysis.TextEdit{{
			Pos:     ifStmt.Pos(),
			End:     ifStmt.End(),
			NewText: []byte(text),
		}}, true
	}
	indent, ok := indent(pass, ifStmt.Pos())
	if !ok {
		return nil, false
	}
	block := "{\n" + indent + "\t" + text + "\n" + indent + "}"
	return []analysis.TextEdit{{
		Pos:     ifStmt.Pos(),
		End:     ifStmt.End(),
		NewText: []byte(block),
	}}, true
}

// commented reports whether a comment sits in the text the fix replaces, or
// trails the closing brace where the delete would leave it describing the
// check. A comment between else and its if would similarly survive.
func commented(pass *analysis.Pass, file *ast.File, ifStmt *ast.IfStmt, elsePos token.Pos) bool {
	for _, group := range file.Comments {
		if elsePos.IsValid() {
			if group.Pos() < ifStmt.Pos() && group.End() > elsePos {
				return true
			}
		}
		if group.Pos() < ifStmt.End() {
			if group.End() > ifStmt.Pos() {
				return true
			}
			if sameLine(pass.Fset, group.End(), ifStmt.Pos()) {
				return true
			}
			continue
		}
		if sameLine(pass.Fset, group.Pos(), ifStmt.End()) {
			return true
		}
	}
	return false
}

// fits reports whether the line the fix writes stays within the length limit.
// A line already past the limit fits: refusing the fix cannot shorten it, and
// linelen reports the line either way. With no limit set, every line fits.
func fits(pass *analysis.Pass, ifStmt *ast.IfStmt, text string, chained bool) bool {
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
	start := lineStart(file, ifStmt.Pos())
	from := file.Offset(ifStmt.Pos())
	to := file.Offset(ifStmt.End())
	first := lineStop(file, src, ifStmt.Pos())
	last := lineStop(file, src, ifStmt.End())
	if start < 0 || from < start || first < from {
		return false
	}
	if last < to || to < from || last > len(src) {
		return false
	}
	if !linelen.CheckLine(src[start:first]) {
		return true
	}
	if chained {
		indent := src[start:from]
		indent = indent[:len(indent)-len(bytes.TrimLeft(indent, " \t"))]
		return linelen.CheckLine([]byte(string(indent) + "\t" + text))
	}
	line := append([]byte(nil), src[start:from]...)
	line = append(line, text...)
	line = append(line, src[to:last]...)
	return linelen.CheckLine(line)
}

// sameLine reports whether a and b are written on the same physical line.
func sameLine(fset *token.FileSet, a, b token.Pos) bool {
	return fset.PositionFor(a, false).Line == fset.PositionFor(b, false).Line
}

// lineStart returns the offset the physical line holding pos starts at.
func lineStart(file *token.File, pos token.Pos) int {
	return file.Offset(file.LineStart(file.PositionFor(pos, false).Line))
}

// lineStop returns the offset of the newline that ends the line pos is on, or
// the length of the text for a last line without one.
func lineStop(file *token.File, src []byte, pos token.Pos) int {
	line := file.PositionFor(pos, false).Line
	if line == file.LineCount() {
		return len(src)
	}
	return file.Offset(file.LineStart(line+1)) - 1
}

// indent returns the whitespace that starts the line pos is on.
func indent(pass *analysis.Pass, pos token.Pos) (string, bool) {
	file := pass.Fset.File(pos)
	if file == nil {
		return "", false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return "", false
	}
	start := lineStart(file, pos)
	end := file.Offset(pos)
	if start < 0 || end < start || end > len(src) {
		return "", false
	}
	line := src[start:end]
	var i int
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return string(line[:i]), true
}
