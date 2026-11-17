// Package deviatingelse provides an analysis.Analyzer that reports an if/else
// whose else block does not fall through to the statement after the if.
//
// The Code Review Comments say to keep the normal code path at the lowest
// indentation (https://go.dev/wiki/CodeReviewComments#indent-error-flow). The
// rule applies when the else block ends in return, break, continue, goto, a
// call to panic, os.Exit, or a package-level Fatal or Panic function in log,
// or an if whose two branches both leave the block. A labeled statement or a
// bare block that ends in one of these forms counts too. Return, goto, and
// panic are terminating statements
// (https://go.dev/ref/spec#Terminating_statements). A break exits the
// innermost for, switch, or select, and a continue begins the next iteration
// of the innermost for (https://go.dev/ref/spec#Break_statements,
// https://go.dev/ref/spec#Continue_statements). Inverting the condition puts
// the else block in the if body and the then block after the if, which
// removes a level of indentation.
//
// An else-if chain is left alone: inverting one link moves its block out of
// the chain. An if with an init statement is left alone, because the then
// block would leave the scope the init declares. An empty then block is left
// alone, because it has nothing to outdent. A then block that ends in one of
// those forms is left alone. errflow reports the return, break, continue,
// goto, and panic cases, and ifreturn reports a then block that only returns.
// A then block ending in a switch, select, or for is reported. For the else
// block, the rule does not cover a switch, select, or for that never falls
// through, so those are left alone.
//
// The fix writes the opposite operator: < and >= swap, <= and > swap, and ==
// and != swap. It removes the ! from a negation and puts one in front of any
// other condition, inside parentheses when that condition is binary.
// Parentheses around the condition are dropped. An ordering comparison that a
// NaN can reach is negated with the !, because a NaN is unordered against
// every value and !(a < b) is true where a >= b is not.
//
// The fix is withheld when the then block declares a name directly, because
// the declaration would move into the enclosing scope. It is also withheld
// when the then block holds a raw string that spans lines, since outdenting
// would change its value, when either block's first statement shares the
// opening brace's line or its last statement shares the closing brace's line,
// when a comment follows the else block's closing brace on the same line,
// because the rewrite would move it onto the last then statement, or when a
// comment would be removed: before or after the condition, between a ! and its
// operand, in the parentheses the fix drops, or between the then block and the
// else block.
//
// The rule is inspired by revive's early-return.
package deviatingelse

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "deviatingelse",
	Doc:  "reports an if/else whose else block does not fall through",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		links := elseIfLinks(file)
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok || links[stmt] || !reportable(pass, stmt) {
				return true
			}
			block := stmt.Else.(*ast.BlockStmt)
			diagnostic := analysis.Diagnostic{
				Pos:     stmt.Else.Pos(),
				End:     block.End(),
				Message: "else does not fall through; invert the condition and outdent the then block",
			}
			if edits, ok := invert(pass, file, stmt, block); ok {
				diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
					Message:   "Invert the condition",
					TextEdits: edits,
				}}
			}
			pass.Report(diagnostic)
			return true
		})
	}
	return nil, nil
}

// elseIfLinks returns the if statements that follow an else.
func elseIfLinks(file *ast.File) map[*ast.IfStmt]bool {
	out := make(map[*ast.IfStmt]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.IfStmt)
		if ok {
			if next, ok := stmt.Else.(*ast.IfStmt); ok {
				out[next] = true
			}
		}
		return true
	})
	return out
}

// reportable reports whether stmt has an else block that leaves the block and
// a then block that does not.
func reportable(pass *analysis.Pass, stmt *ast.IfStmt) bool {
	if stmt.Init != nil || stmt.Else == nil {
		return false
	}
	block, ok := stmt.Else.(*ast.BlockStmt)
	if !ok || len(stmt.Body.List) == 0 {
		return false
	}
	if terminates(pass, stmt.Body) {
		return false
	}
	return terminates(pass, block)
}

// terminates reports whether control cannot reach the end of block.
func terminates(pass *analysis.Pass, block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	return ends(pass, block.List[len(block.List)-1])
}

// ends reports whether stmt leaves the block it ends.
func ends(pass *analysis.Pass, stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return stmt.Tok != token.FALLTHROUGH
	case *ast.ExprStmt:
		return exits(pass, stmt.X)
	case *ast.IfStmt:
		return stmt.Else != nil && terminates(pass, stmt.Body) &&
			terminates(pass, asBlock(stmt.Else))
	case *ast.LabeledStmt:
		return ends(pass, stmt.Stmt)
	case *ast.BlockStmt:
		return terminates(pass, stmt)
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

// exits reports whether e is a call that does not return.
func exits(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		obj := pass.TypesInfo.Uses[fun]
		if obj == types.Universe.Lookup("panic") {
			return true
		}
		return exitObject(obj)
	case *ast.SelectorExpr:
		return exitObject(pass.TypesInfo.Uses[fun.Sel])
	}
	return false
}

// exitObject reports whether obj is a package-level os.Exit or log Fatal or
// Panic function.
func exitObject(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Signature().Recv() != nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "os":
		return fn.Name() == "Exit"
	case "log":
		switch fn.Name() {
		case "Fatal", "Fatalf", "Fatalln", "Panic", "Panicf", "Panicln":
			return true
		}
	}
	return false
}

// invert returns the edit that inverts stmt's condition and moves the then
// block after the if.
func invert(pass *analysis.Pass, file *ast.File, stmt *ast.IfStmt, elseBlock *ast.BlockStmt) ([]analysis.TextEdit, bool) {
	tokFile, src, ok := readSource(pass, stmt)
	if !ok || !editable(file, tokFile, stmt, elseBlock) {
		return nil, false
	}
	cond := negate(pass, src, tokFile, stmt.Cond)
	parent, unit, ok := indentation(src, tokFile, stmt, elseBlock)
	if !ok {
		return nil, false
	}
	base := append(bytes.Clone(parent), unit...)
	elseTxt := blockLines(src, tokFile, elseBlock)
	thenTxt := blockLines(src, tokFile, stmt.Body)
	inlineComment(elseTxt, parent, base)
	inlineComment(thenTxt, parent, base)
	for i, line := range thenTxt {
		thenTxt[i] = strings.TrimPrefix(line, string(unit))
	}
	var text bytes.Buffer
	text.WriteString("if ")
	text.Write(cond)
	text.WriteString(" {\n")
	for _, line := range elseTxt {
		text.WriteString(line)
		text.WriteByte('\n')
	}
	text.Write(parent)
	text.WriteString("}")
	for _, line := range thenTxt {
		text.WriteByte('\n')
		text.WriteString(line)
	}
	return []analysis.TextEdit{{
		Pos:     stmt.Pos(),
		End:     stmt.End(),
		NewText: text.Bytes(),
	}}, true
}

// readSource returns the file holding stmt and its source.
func readSource(pass *analysis.Pass, stmt *ast.IfStmt) (*token.File, []byte, bool) {
	tokFile := pass.Fset.File(stmt.Pos())
	if tokFile == nil {
		return nil, nil, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return nil, nil, false
	}
	return tokFile, src, true
}

// editable reports whether the fix can rewrite stmt without changing a name, a
// raw string, or a comment, and with no statement sharing a brace line.
func editable(file *ast.File, tokFile *token.File, stmt *ast.IfStmt, elseBlock *ast.BlockStmt) bool {
	return movable(stmt) && uncommented(file, stmt, elseBlock) &&
		!onBraceLines(tokFile, stmt.Body) &&
		!onBraceLines(tokFile, elseBlock) &&
		!trailing(file, tokFile, elseBlock.Rbrace)
}

// movable reports whether stmt's then block can move into the enclosing block
// without changing what one of its names means or what a raw string holds.
func movable(stmt *ast.IfStmt) bool {
	return !declares(stmt.Body) && !hasRawString(stmt.Body)
}

// uncommented reports whether the rewrite would keep every comment.
func uncommented(file *ast.File, stmt *ast.IfStmt, elseBlock *ast.BlockStmt) bool {
	var (
		before = commented(file, stmt.Pos(), stmt.Cond.Pos())
		after  = commented(file, stmt.Cond.End(), stmt.Body.Lbrace)
		gap    = commented(file, stmt.Body.Rbrace+1, elseBlock.Lbrace)
	)
	return !before && !after && !gap && !lostComment(file, stmt.Cond)
}

// lostComment reports whether cond holds a comment outside the text the fix
// copies.
func lostComment(file *ast.File, cond ast.Expr) bool {
	kept, _ := stripped(cond)
	return commented(file, cond.Pos(), kept.Pos()) ||
		commented(file, kept.End(), cond.End())
}

// trailing reports whether a comment follows pos on the same line.
func trailing(file *ast.File, tokFile *token.File, pos token.Pos) bool {
	line := tokFile.Line(pos)
	for _, group := range file.Comments {
		if group.Pos() > pos && tokFile.Line(group.Pos()) == line {
			return true
		}
	}
	return false
}

// onBraceLines reports whether block's first statement shares the opening
// brace's line or its last statement shares the closing brace's line.
func onBraceLines(tokFile *token.File, block *ast.BlockStmt) bool {
	if len(block.List) == 0 {
		return false
	}
	first, last := block.List[0], block.List[len(block.List)-1]
	return tokFile.Line(first.Pos()) == tokFile.Line(block.Lbrace) ||
		tokFile.Line(last.End()) == tokFile.Line(block.Rbrace)
}

// indentation returns the indent before stmt, and the indent a block adds to
// it.
func indentation(src []byte, tokFile *token.File, stmt *ast.IfStmt, elseBlock *ast.BlockStmt) (parent, unit []byte, ok bool) {
	parent = indentAt(src, tokFile.Offset(stmt.Pos()))
	unit = indentUnitFor(src, tokFile, parent, stmt.Body, elseBlock)
	return parent, unit, len(unit) > 0
}

// inlineComment moves a comment that shares the opening brace's line to base.
func inlineComment(lines []string, parent, base []byte) {
	if len(lines) == 0 || strings.HasPrefix(lines[0], string(parent)) {
		return
	}
	lines[0] = string(base) + strings.TrimLeft(lines[0], " \t")
}

// blockLines returns the lines between block's braces with leading and
// trailing blank lines removed.
func blockLines(src []byte, tokFile *token.File, block *ast.BlockStmt) []string {
	var (
		lo    = tokFile.Offset(block.Lbrace) + 1
		hi    = tokFile.Offset(block.Rbrace)
		lines = strings.Split(string(src[lo:hi]), "\n")
	)
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
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

// indentUnitFor returns the indentation the blocks add to parent, measured
// from the shallowest statement inside them. A label sits one level out from
// the statements around it, so a label is skipped in favor of its statement or
// the next one.
func indentUnitFor(src []byte, tokFile *token.File, parent []byte, blocks ...*ast.BlockStmt) []byte {
	var best []byte
	for _, block := range blocks {
		ast.Inspect(block, func(n ast.Node) bool {
			stmt, ok := n.(ast.Stmt)
			if !ok {
				return true
			}
			if _, label := stmt.(*ast.LabeledStmt); label {
				return true
			}
			inner := indentAt(src, tokFile.Offset(stmt.Pos()))
			if len(inner) <= len(parent) || !bytes.HasPrefix(inner, parent) {
				return true
			}
			if best == nil || len(inner) < len(best) {
				best = inner
			}
			return true
		})
	}
	if best == nil {
		return nil
	}
	return best[len(parent):]
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

// declares reports whether block declares a name directly.
func declares(block *ast.BlockStmt) bool {
	for _, stmt := range block.List {
		switch stmt := unlabel(stmt).(type) {
		case *ast.DeclStmt:
			return true
		case *ast.AssignStmt:
			if stmt.Tok == token.DEFINE {
				return true
			}
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

// negate returns the source of the condition that is true when cond is false.
func negate(pass *analysis.Pass, src []byte, tokFile *token.File, cond ast.Expr) []byte {
	var (
		kept, negated = stripped(cond)
		lo            = tokFile.Offset(kept.Pos())
		text          = src[lo:tokFile.Offset(kept.End())]
	)
	if negated {
		return text
	}
	bin, ok := kept.(*ast.BinaryExpr)
	if !ok {
		return append([]byte("!"), text...)
	}
	if opp, ok := opposite[bin.Op]; ok && !mayBeNaN(pass, bin) {
		return replaced(text, tokFile.Offset(bin.OpPos)-lo, bin.Op, opp)
	}
	out := []byte("!(")
	out = append(out, text...)
	return append(out, ')')
}

// stripped returns the expression the fix writes where cond was, which is cond
// without its parentheses and, for a negation, without the ! and the
// parentheses around its operand.
func stripped(cond ast.Expr) (e ast.Expr, negated bool) {
	cond = ast.Unparen(cond)
	if not, ok := cond.(*ast.UnaryExpr); ok && not.Op == token.NOT {
		return ast.Unparen(not.X), true
	}
	return cond, false
}

// opposite maps a comparison to the one that is true where it is false.
var opposite = map[token.Token]token.Token{
	token.EQL: token.NEQ,
	token.NEQ: token.EQL,
	token.LSS: token.GEQ,
	token.GEQ: token.LSS,
	token.GTR: token.LEQ,
	token.LEQ: token.GTR,
}

// mayBeNaN reports whether a NaN can reach bin's ordering comparison. Under
// IEEE 754 a NaN is unordered against every value, so !(a < b) is true where
// a >= b is not; == and != give the same answer either way.
func mayBeNaN(pass *analysis.Pass, bin *ast.BinaryExpr) bool {
	switch bin.Op {
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
	default:
		return false
	}
	for _, operand := range []ast.Expr{bin.X, bin.Y} {
		tv := pass.TypesInfo.Types[operand]
		// A constant is never a NaN.
		if tv.Value == nil && astexpr.MayBeNaN(tv.Type) {
			return true
		}
	}
	return false
}

// replaced returns text with op at off replaced by opp.
func replaced(text []byte, off int, op, opp token.Token) []byte {
	out := append([]byte{}, text[:off]...)
	out = append(out, opp.String()...)
	return append(out, text[off+len(op.String()):]...)
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
