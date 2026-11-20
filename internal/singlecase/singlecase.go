// Package singlecase holds what the analyzers that remove a select with a
// single case share: the case itself, whether its body breaks out of the
// select, and the edit that replaces the select with the case's statement and
// body.
//
// Removing a select leaves its case body in the block around it, so the fix
// that removes the select carries the body along. Edit withholds the edit when
// there is nothing it can move:
//
//   - the select does not occupy whole lines, so there is no line to replace;
//   - a comment above the case would be deleted.
//
// It withholds the edit too when moving the body would change the program:
//
//   - the body holds a break that leaves the select, and removing the select
//     would change where the break goes;
//   - the body holds a raw string that spans lines, which would take the
//     reindentation as part of its value;
//   - a name in the case is declared in the block around the select or in a
//     scope that block can see, so moving it would shadow what that block
//     reads;
//   - a goto would jump over a declaration the edit moves, which Go rejects;
//   - a name that two sibling selects would both move into that block, which
//     the pair would redeclare. The guard withholds as soon as a sibling
//     declares the name, whether or not that sibling's own edit survives the
//     rest of this list.
package singlecase

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"

	"lesiw.io/tools/internal/astexpr"
	"lesiw.io/tools/internal/source"
)

// Single returns the only case of sel, when sel has exactly one and it is not
// a default clause.
func Single(sel *ast.SelectStmt) (*ast.CommClause, bool) {
	if len(sel.Body.List) != 1 {
		return nil, false
	}
	clause, ok := sel.Body.List[0].(*ast.CommClause)
	if !ok || clause.Comm == nil {
		return nil, false
	}
	return clause, true
}

// Unwrap returns Edit with the case's own send or receive as head.
func Unwrap(pass *analysis.Pass, src []byte, file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) (analysis.TextEdit, bool) {
	var (
		tokFile = pass.Fset.File(sel.Pos())
		start   = tokFile.Offset(clause.Comm.Pos())
		end     = tokFile.Offset(clause.Comm.End())
	)
	return Edit(pass, src, file, sel, clause, string(src[start:end]))
}

// Edit returns the edit that replaces sel and its lines with head followed by
// the case body, and false when the edit would lose a comment or change the
// program.
func Edit(pass *analysis.Pass, src []byte, file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause, head string) (analysis.TextEdit, bool) {
	var (
		tokFile   = pass.Fset.File(sel.Pos())
		selStart  = tokFile.Offset(sel.Pos())
		selEnd    = tokFile.Offset(sel.End())
		lineStart = source.Start(src, selStart)
		lineEnd   = source.End(src, selEnd)
	)
	if !source.Alone(src, selStart, selEnd) || commented(file, sel, clause) {
		return analysis.TextEdit{}, false
	}
	if Leaves(file, sel, clause) || rawString(clause.Body) {
		return analysis.TextEdit{}, false
	}
	if redeclares(pass, sel, clause) || clashes(pass, file, sel, clause) {
		return analysis.TextEdit{}, false
	}
	if jumpsOver(file, sel, decls(clause)) {
		return analysis.TextEdit{}, false
	}
	bodyStart := tokFile.Offset(clause.Colon) + 1
	bodyEnd := tokFile.Offset(sel.Body.Rbrace)
	indent := string(src[lineStart:selStart])
	body := src[bodyStart:bodyEnd]
	return analysis.TextEdit{
		Pos:     tokFile.Pos(lineStart),
		End:     tokFile.Pos(lineEnd),
		NewText: []byte(indent + head + "\n" + reindent(body, indent)),
	}, true
}

// commented reports whether the text that head replaces holds a comment.
func commented(file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) bool {
	return astexpr.Commented(file, sel.Pos(), clause.Comm.Pos()) ||
		astexpr.Commented(file, clause.Comm.End(), clause.Colon+1)
}

// Leaves reports whether the body of clause holds a break that leaves sel.
func Leaves(file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) bool {
	return scan(clause.Body, 0, labelOf(file, sel))
}

// labelOf returns the name of the label on sel, or the empty string.
func labelOf(file *ast.File, sel *ast.SelectStmt) string {
	path, _ := astutil.PathEnclosingInterval(file, sel.Pos(), sel.End())
	for _, node := range path {
		labeled, ok := node.(*ast.LabeledStmt)
		if ok && labeled.Stmt == sel {
			return labeled.Label.Name
		}
	}
	return ""
}

// scan reports whether stmts holds a break that leaves the select, with depth
// counting the loops, switches, and selects around the break.
func scan(stmts []ast.Stmt, depth int, label string) bool {
	for _, stmt := range stmts {
		if scanStmt(stmt, depth, label) {
			return true
		}
	}
	return false
}

// scanStmt reports whether stmt holds a break that leaves the select.
func scanStmt(stmt ast.Stmt, depth int, label string) bool {
	switch s := stmt.(type) {
	case *ast.BranchStmt:
		return leaves(s, depth, label)
	case *ast.LabeledStmt:
		return scanStmt(s.Stmt, depth, label)
	case *ast.BlockStmt:
		return scan(s.List, depth, label)
	case *ast.IfStmt:
		if scanStmt(s.Body, depth, label) {
			return true
		}
		if s.Else != nil {
			return scanStmt(s.Else, depth, label)
		}
		return false
	case *ast.ForStmt:
		return scanStmt(s.Body, depth+1, label)
	case *ast.RangeStmt:
		return scanStmt(s.Body, depth+1, label)
	case *ast.SwitchStmt:
		return scan(s.Body.List, depth+1, label)
	case *ast.TypeSwitchStmt:
		return scan(s.Body.List, depth+1, label)
	case *ast.SelectStmt:
		return scan(s.Body.List, depth+1, label)
	case *ast.CaseClause:
		return scan(s.Body, depth, label)
	case *ast.CommClause:
		return scan(s.Body, depth, label)
	}
	return false
}

// leaves reports whether branch leaves the select. An unlabeled break leaves
// the innermost loop, switch, or select, which is the select only at depth
// zero. A labeled break leaves the statement its label names.
func leaves(branch *ast.BranchStmt, depth int, label string) bool {
	if branch.Tok != token.BREAK {
		return false
	}
	if branch.Label != nil {
		return label != "" && branch.Label.Name == label
	}
	return depth == 0
}

// rawString reports whether body holds a raw string that spans lines. Its
// continuation lines belong to the value, so removing indentation would
// change it.
func rawString(body []ast.Stmt) bool {
	var found bool
	ast.Inspect(&ast.BlockStmt{List: body}, func(n ast.Node) bool {
		if found {
			return false
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		multiline := strings.Contains(lit.Value, "\n")
		if multiline && strings.HasPrefix(lit.Value, "`") {
			found = true
			return false
		}
		return true
	})
	return found
}

// redeclares reports whether moving the case's declarations into the block
// around sel would take a name from a scope that block can see.
func redeclares(pass *analysis.Pass, sel *ast.SelectStmt, clause *ast.CommClause) bool {
	var (
		outer = pass.Pkg.Scope().Innermost(sel.Pos())
		inner = pass.TypesInfo.Scopes[clause]
	)
	if outer == nil || inner == nil {
		return true
	}
	for _, name := range inner.Names() {
		if outer.Lookup(name) != nil {
			return true
		}
		if _, obj := outer.LookupParent(name, sel.Pos()); obj != nil {
			return true
		}
	}
	return false
}

// clashes reports whether another select in the same block would move a name
// that clause moves into that block, where the two edits would redeclare it.
func clashes(pass *analysis.Pass, file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) bool {
	moved := declaredNames(pass, clause)
	if len(moved) == 0 {
		return false
	}
	owner := container(file, sel)
	if owner == nil {
		return true
	}
	for _, stmt := range statements(owner) {
		other := selectOf(stmt)
		if other == nil || other == sel {
			continue
		}
		sibling, ok := Single(other)
		if !ok {
			continue
		}
		for _, name := range declaredNames(pass, sibling) {
			if slices.Contains(moved, name) {
				return true
			}
		}
	}
	return false
}

// declaredNames returns the names the case declares in its own block.
func declaredNames(pass *analysis.Pass, clause *ast.CommClause) []string {
	scope := pass.TypesInfo.Scopes[clause]
	if scope == nil {
		return nil
	}
	return scope.Names()
}

// statements returns the statement list of a block or a clause.
func statements(node ast.Node) []ast.Stmt {
	switch node := node.(type) {
	case *ast.BlockStmt:
		return node.List
	case *ast.CaseClause:
		return node.Body
	case *ast.CommClause:
		return node.Body
	}
	return nil
}

// selectOf returns stmt when it is a select, looking through its labels.
func selectOf(stmt ast.Stmt) *ast.SelectStmt {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			break
		}
		stmt = labeled.Stmt
	}
	sel, _ := stmt.(*ast.SelectStmt)
	return sel
}

// jumpsOver reports whether a goto before sel or inside its body jumps over a
// var declaration that the fix moves out of the case to a label after sel.
func jumpsOver(file *ast.File, sel *ast.SelectStmt, decls []token.Pos) bool {
	owner := container(file, sel)
	if owner == nil {
		// With no statement list to inspect, a declaration the edit moves
		// cannot be cleared.
		return len(decls) > 0
	}
	var gotos []*ast.BranchStmt
	labels := make(map[string]token.Pos)
	ast.Inspect(owner, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			labels[n.Label.Name] = n.Label.Pos()
		case *ast.BranchStmt:
			if n.Tok == token.GOTO {
				gotos = append(gotos, n)
			}
		}
		return true
	})
	for _, jump := range gotos {
		target, ok := labels[jump.Label.Name]
		if !ok || target <= sel.End() || jump.Pos() >= sel.End() {
			continue
		}
		for _, decl := range decls {
			if decl > jump.Pos() && decl < target {
				return true
			}
		}
	}
	return false
}

// decls returns the positions of the var declarations in clause that move into
// the block around sel.
func decls(clause *ast.CommClause) []token.Pos {
	var (
		out []token.Pos
		add = func(stmt ast.Stmt) {
			for {
				labeled, ok := stmt.(*ast.LabeledStmt)
				if !ok {
					break
				}
				stmt = labeled.Stmt
			}
			switch s := stmt.(type) {
			case *ast.DeclStmt:
				gen, ok := s.Decl.(*ast.GenDecl)
				if ok && gen.Tok == token.VAR {
					out = append(out, gen.Pos())
				}
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					out = append(out, s.Pos())
				}
			}
		}
	)
	add(clause.Comm)
	for _, stmt := range clause.Body {
		add(stmt)
	}
	return out
}

// container returns the node whose statement list holds sel.
func container(file *ast.File, sel *ast.SelectStmt) ast.Node {
	path, _ := astutil.PathEnclosingInterval(file, sel.Pos(), sel.End())
	for _, node := range path {
		for _, stmt := range statements(node) {
			if holds(stmt, sel) {
				return node
			}
		}
	}
	return nil
}

// holds reports whether stmt is sel or a label on sel.
func holds(stmt ast.Stmt, sel *ast.SelectStmt) bool {
	if stmt == sel {
		return true
	}
	labeled, ok := stmt.(*ast.LabeledStmt)
	return ok && labeled.Stmt == sel
}

// reindent returns the statements in body with one level of indentation
// removed, so that the body is indented at indent.
func reindent(body []byte, indent string) string {
	var (
		lines = strings.Split(string(body), "\n")
		out   = make([]string, 0, len(lines))
	)
	if first := strings.TrimSpace(lines[0]); first != "" {
		out = append(out, indent+first)
	}
	for _, line := range lines[1:] {
		out = append(out, strings.TrimPrefix(line, "\t"))
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}
