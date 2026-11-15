// Package unlabelstmt provides an analysis.Analyzer that reports a redundant
// label.
//
// A bare break leaves the innermost for, switch, or select, and a bare
// continue begins the next iteration of the innermost for. The analyzer
// reports a label when no goto names it and every break or continue that names
// it would apply to the same statement without the name.
//
// The blank label is reported too: nothing can name it. The compiler requires
// a use for every other label.
//
// When the label is alone on its line, the fix deletes the whole line. A break
// or continue that names the label becomes bare.
//
// Like go-critic's unlabelStmt.
package unlabelstmt

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "unlabelstmt",
	Doc:  "reports a redundant label",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tokFile := pass.Fset.File(file.FileStart)
		src, err := pass.ReadFile(tokFile.Name())
		if err != nil {
			return nil, err
		}
		uses := labelUses(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			labeled, ok := n.(*ast.LabeledStmt)
			if !ok {
				return true
			}
			// A blank label has no object, so no branch can name it.
			refs := uses[pass.TypesInfo.Defs[labeled.Label]]
			if redundant(file, labeled.Stmt, refs) {
				pass.Report(diagnostic(tokFile, src, labeled, refs))
			}
			return true
		})
	}
	return nil, nil
}

// labelUses maps each label in file to the branches that name it. The key is
// the label object, which keeps two functions' labels of the same name apart.
func labelUses(pass *analysis.Pass, file *ast.File) map[types.Object][]*ast.BranchStmt {
	uses := make(map[types.Object][]*ast.BranchStmt)
	ast.Inspect(file, func(n ast.Node) bool {
		branch, ok := n.(*ast.BranchStmt)
		if !ok || branch.Label == nil {
			return true
		}
		label := pass.TypesInfo.Uses[branch.Label]
		uses[label] = append(uses[label], branch)
		return true
	})
	return uses
}

// redundant reports whether nothing needs the label on stmt. A goto needs the
// label it names, and a break or continue needs it when dropping the name
// would send it somewhere else.
func redundant(file *ast.File, stmt ast.Stmt, refs []*ast.BranchStmt) bool {
	for _, branch := range refs {
		if branch.Tok == token.GOTO || bareTarget(file, branch) != stmt {
			return false
		}
	}
	return true
}

// bareTarget returns the statement that branch would reach without its label.
func bareTarget(file *ast.File, branch *ast.BranchStmt) ast.Stmt {
	path, _ := astutil.PathEnclosingInterval(file, branch.Pos(), branch.End())
	for _, node := range path {
		stmt, ok := node.(ast.Stmt)
		if !ok {
			continue
		}
		switch stmt.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return stmt
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if branch.Tok == token.BREAK {
				return stmt
			}
		}
	}
	return nil
}

// diagnostic returns the report for one label, with an edit that removes the
// label and one that removes its name from each branch that names it.
func diagnostic(tokFile *token.File, src []byte, labeled *ast.LabeledStmt, refs []*ast.BranchStmt) analysis.Diagnostic {
	edits := make([]analysis.TextEdit, 0, len(refs)+1)
	edits = append(edits, removeLabel(tokFile, src, labeled))
	for _, branch := range refs {
		edits = append(edits, analysis.TextEdit{
			Pos: branch.Label.Pos(),
			End: branch.Label.End(),
		})
	}
	return analysis.Diagnostic{
		Pos:     labeled.Label.Pos(),
		End:     labeled.Label.End(),
		Message: "redundant label " + labeled.Label.Name,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "Drop the label",
			TextEdits: edits,
		}},
	}
}

// removeLabel returns the edit that drops the label. A label alone on its line
// takes the line, since dropping only the name would leave the line blank.
func removeLabel(tokFile *token.File, src []byte, labeled *ast.LabeledStmt) analysis.TextEdit {
	var (
		pos   = labeled.Label.Pos()
		line  = tokFile.Line(pos)
		start = tokFile.Offset(pos)
		end   = tokFile.Offset(labeled.Colon) + 1
		tail  = len(src)
	)
	// A label can share the last line of a file with its statement, and
	// LineStart panics on the line after that one.
	if line < tokFile.LineCount() {
		tail = tokFile.Offset(tokFile.LineStart(line + 1))
	}
	head := tokFile.Offset(tokFile.LineStart(line))
	if onlySpace(src[end:tail]) && onlySpace(src[head:start]) {
		start, end = head, tail
	}
	return analysis.TextEdit{Pos: tokFile.Pos(start), End: tokFile.Pos(end)}
}

// onlySpace reports whether b holds nothing but whitespace.
func onlySpace(b []byte) bool { return strings.TrimSpace(string(b)) == "" }
