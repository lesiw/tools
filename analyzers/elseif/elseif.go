// Package elseif provides an analysis.Analyzer that reports an if nested in
// an else block.
//
// else { if ... } and else if ... are the same program. The second form
// removes a pair of braces and one level of indent.
//
// The fix carries the comments the rewrite would delete. A comment on the
// else line stays at the end of the else if line while that line stays within
// linelen's limit. Any other carried comment moves into the body, above the
// branch's first statement, where it reads as a comment on that statement.
// The fix is withheld when a carried comment cannot stay inside the statement:
// one that follows the nested if, or one that is a directive, which a tool
// reads at its position.
//
// The rule is like go-critic's elseif.
package elseif

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/source"
)

var Analyzer = &analysis.Analyzer{
	Name: "elseif",
	Doc:  "reports an if nested in an else block",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			block, ok := stmt.Else.(*ast.BlockStmt)
			if !ok || len(block.List) != 1 {
				return true
			}
			inner, ok := block.List[0].(*ast.IfStmt)
			if !ok {
				return true
			}
			diagnostic := analysis.Diagnostic{
				Pos:     stmt.Else.Pos(),
				End:     block.End(),
				Message: "if nested in an else; write else if",
			}
			if edits, safe := collapse(pass, file, stmt, inner); safe {
				diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
					Message:   "Write else if",
					TextEdits: edits,
				}}
			}
			pass.Report(diagnostic)
			return true
		})
	}
	return nil, nil
}

// collapse returns the edits that write else if, and reports whether the
// rewrite finds a place for every comment it deletes.
func collapse(pass *analysis.Pass, file *ast.File, stmt, inner *ast.IfStmt) ([]analysis.TextEdit, bool) {
	before, safe := dropped(file, stmt, inner)
	if !safe {
		return nil, false
	}
	edits := []analysis.TextEdit{{Pos: stmt.Else.Pos(), End: inner.Pos()}}
	if len(before) > 0 {
		carried, safe := carry(pass, stmt, inner, before)
		if !safe {
			return nil, false
		}
		edits = append(edits, carried)
	}
	last := analysis.TextEdit{Pos: inner.End(), End: stmt.Else.End()}
	return append(edits, last), true
}

// dropped returns the comments the rewrite deletes, and reports whether it
// can carry each of them. A comment needs a line to land on, and it must not
// be a directive.
func dropped(file *ast.File, stmt, inner *ast.IfStmt) ([]*ast.CommentGroup, bool) {
	// A comment that follows the nested if is the last thing in the else
	// block, and the else if has no line there.
	if commented(file, inner.End(), stmt.Else.End()) {
		return nil, false
	}
	var before []*ast.CommentGroup
	for _, group := range file.Comments {
		if group.Pos() >= inner.Pos() || group.End() <= stmt.Else.Pos() {
			continue
		}
		for _, comment := range group.List {
			if directive(comment.Text) {
				return nil, false
			}
		}
		before = append(before, group)
	}
	return before, true
}

// carry returns the edit that writes the comments before the nested if into
// the else if, and reports whether it could build one.
func carry(pass *analysis.Pass, stmt, inner *ast.IfStmt, before []*ast.CommentGroup) (analysis.TextEdit, bool) {
	tokFile := pass.Fset.File(inner.Pos())
	if tokFile == nil {
		return analysis.TextEdit{}, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	// The offsets index the bytes the driver parsed, so a file that cannot be
	// read, or that changed size, is not used.
	if err != nil || tokFile.Size() != len(src) {
		return analysis.TextEdit{}, false
	}
	var onElse, above []string
	line := tokFile.Line(stmt.Else.Pos())
	for _, group := range before {
		text := sourceText(src, tokFile, group.Pos(), group.End())
		if onLine(tokFile, group, line) {
			onElse = append(onElse, text)
		} else {
			above = append(above, text)
		}
	}
	var pieces []string
	if len(onElse) > 0 {
		trailing := " " + strings.Join(onElse, " ")
		if fitsLine(src, tokFile, stmt, inner, trailing) {
			pieces = append(pieces, trailing)
		} else {
			above = append(onElse, above...)
		}
	}
	for _, text := range above {
		pieces = append(pieces, "\n"+text)
	}
	brace := tokFile.Offset(inner.Body.Lbrace) + 1
	if !source.Blank(src[brace:source.End(src, brace)]) {
		// The body continues on this line, so the last carried comment has to
		// end it.
		pieces = append(pieces, "\n")
	}
	return analysis.TextEdit{
		Pos:     inner.Body.Lbrace + 1,
		NewText: []byte(strings.Join(pieces, "")),
	}, true
}

// sourceText returns the source text between lo and hi.
func sourceText(src []byte, tokFile *token.File, lo, hi token.Pos) string {
	return string(src[tokFile.Offset(lo):tokFile.Offset(hi)])
}

// commented reports whether a comment overlaps the half-open range lo to hi.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}

// directive reports whether text is a comment a tool reads at its position. A
// directive is a line comment with no space after its slashes, and the block
// form of a line directive. A list of names would miss a tool's spelling; a
// comment with no space is not prose to this suite, whatever reads it.
func directive(text string) bool {
	if strings.HasPrefix(text, "/*line ") {
		return true
	}
	return strings.HasPrefix(text, "//") && len(text) > 2 && text[2] != ' '
}

// onLine reports whether group starts and ends on line.
func onLine(tokFile *token.File, group *ast.CommentGroup, line int) bool {
	return tokFile.Line(group.Pos()) == line &&
		tokFile.Line(group.End()) == line
}

// fitsLine reports whether the else if line, with comment appended, is within
// linelen's limit. A header that spans lines does not fit: the reconstruction
// assumes one line, and the formatter reindents what is left.
func fitsLine(src []byte, tokFile *token.File, stmt, inner *ast.IfStmt, comment string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	if tokFile.Line(inner.Pos()) != tokFile.Line(inner.Body.Lbrace) {
		return false
	}
	var (
		head   = source.Start(src, tokFile.Offset(stmt.Body.Rbrace))
		rbrace = tokFile.Offset(stmt.Else.Pos())
		brace  = tokFile.Offset(inner.Body.Lbrace) + 1
		line   = string(src[head:rbrace]) +
			string(src[tokFile.Offset(inner.Pos()):brace])
	)
	return linelen.CheckLine([]byte(line + comment))
}
