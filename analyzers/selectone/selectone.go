// Package selectone provides an analysis.Analyzer that reports a select
// statement with a single case clause and no default.
//
// The select statements section of the Go spec says a select with no default
// "blocks until at least one of the communications can proceed"
// (https://go.dev/ref/spec#Select_statements). With one case, that is the send
// or receive in the case, which blocks the same way. Removing the select
// leaves the send or receive and the body.
//
// A select with no cases, a default clause, or more than one case is left
// alone. A select whose case body holds a break that leaves the select is
// left alone too: the break statement section says a break ends the innermost
// for, switch, or select, or the statement named by a label
// (https://go.dev/ref/spec#Break_statements), and removing the select would
// change where the break goes.
//
// Safe fix: replace the select with the send or receive in the case and the
// body. The fix is withheld when it would delete a comment, when the select
// does not occupy whole lines, when the body holds a raw string that spans
// lines, when a name in the case is declared anywhere in the block around the
// select or in a scope that block can see, or when a goto would jump over a
// var declaration the fix moves out of the case.
//
// The rule is adapted from staticcheck's S1000, which reports a single-case
// select and offers no fix.
package selectone

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"

	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "selectone",
	Doc:  "reports a select statement with a single case and no default",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		src, err := pass.ReadFile(name)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectStmt)
			if !ok || len(sel.Body.List) != 1 {
				return true
			}
			clause, ok := sel.Body.List[0].(*ast.CommClause)
			if !ok || clause.Comm == nil {
				return true
			}
			if breaks(clause.Body, labelOf(file, sel)) {
				return true
			}
			pass.Report(diagnostic(pass, src, file, sel, clause))
			return true
		})
	}
	return nil, nil
}

// diagnostic returns the report for one select, with a fix when the select can
// be removed without changing the program.
func diagnostic(pass *analysis.Pass, src []byte, file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) analysis.Diagnostic {
	d := analysis.Diagnostic{
		Pos:     sel.Pos(),
		End:     sel.End(),
		Message: "select with a single case is redundant",
	}
	if edit, ok := unwrap(pass, src, file, sel, clause); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the select",
			TextEdits: []analysis.TextEdit{edit},
		}}
	}
	return d
}

// unwrap returns the edit that replaces sel with the communication and body of
// its case, and false when the edit would not preserve the program.
func unwrap(pass *analysis.Pass, src []byte, file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) (analysis.TextEdit, bool) {
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
	if rawString(clause.Body) {
		return analysis.TextEdit{}, false
	}
	if redeclares(pass, sel, clause) {
		return analysis.TextEdit{}, false
	}
	if jumpsOver(file, sel, decls(clause)) {
		return analysis.TextEdit{}, false
	}
	commStart := tokFile.Offset(clause.Comm.Pos())
	commEnd := tokFile.Offset(clause.Comm.End())
	bodyStart := tokFile.Offset(clause.Colon) + 1
	bodyEnd := tokFile.Offset(sel.Body.Rbrace)
	indent := string(src[lineStart:selStart])
	comm := src[commStart:commEnd]
	body := src[bodyStart:bodyEnd]
	text := indent + string(comm) + "\n" + bodyText(body, indent)
	return analysis.TextEdit{
		Pos:     tokFile.Pos(lineStart),
		End:     tokFile.Pos(lineEnd),
		NewText: []byte(text),
	}, true
}

// commented reports whether sel holds a comment that the fix would delete.
func commented(file *ast.File, sel *ast.SelectStmt, clause *ast.CommClause) bool {
	for _, group := range file.Comments {
		if group.Pos() < clause.Comm.Pos() && group.End() > sel.Pos() {
			return true
		}
		if group.Pos() < clause.Colon+1 && group.End() > clause.Comm.End() {
			return true
		}
	}
	return false
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

// breaks reports whether body holds a break that leaves the select. label is
// the name of the label on the select, or the empty string.
func breaks(body []ast.Stmt, label string) bool {
	return scan(body, 0, label)
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

// jumpsOver reports whether a goto before sel or inside its body jumps over a
// var declaration that the fix moves out of the case to a label after sel.
func jumpsOver(file *ast.File, sel *ast.SelectStmt, decls []token.Pos) bool {
	owner := container(file, sel)
	if owner == nil {
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

// container returns the node whose statement list holds sel.
func container(file *ast.File, sel *ast.SelectStmt) ast.Node {
	path, _ := astutil.PathEnclosingInterval(file, sel.Pos(), sel.End())
	for _, node := range path {
		switch n := node.(type) {
		case *ast.BlockStmt:
			for _, stmt := range n.List {
				if holds(stmt, sel) {
					return n
				}
			}
		case *ast.CaseClause:
			for _, stmt := range n.Body {
				if holds(stmt, sel) {
					return n
				}
			}
		case *ast.CommClause:
			for _, stmt := range n.Body {
				if holds(stmt, sel) {
					return n
				}
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

// bodyText returns the statements in body with one level of indentation
// removed, so that the body is indented at indent.
func bodyText(body []byte, indent string) string {
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
