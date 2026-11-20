// Package dupbuildtag provides an analysis.Analyzer that reports a // +build
// line whose condition an earlier line in the same file already gives.
//
// The build-constraint section of the go command's documentation says "It is
// an error for a file to have more than one //go:build line"
// (https://pkg.go.dev/cmd/go#hdr-Build_constraints). The design document for
// the newer syntax says multiple legacy // +build lines "were in effect AND'ed
// together"
// (https://go.googlesource.com/proposal/+/master/design/draft-gobuild.md). A
// line that repeats an earlier one therefore changes nothing and is usually a
// copy-paste error.
//
// Only // +build lines are read. The go command rejects a second //go:build
// line in a file it builds, and the suite's buildtag pass reports one in a
// file the build ignores, so this rule does not read //go:build lines. A
// // +build line is read whether or not the file also holds a //go:build
// line: a repeated line is still a copy-paste error, and plusbuild reports
// only the first obsolete line in a group. A line with an invalid tag is left
// alone, and a generated file is left alone since an edit would change
// generated text.
//
// The go command reads a legacy line only in the header: the run of blank
// lines and // comments before the first non-//, non-blank line. A // +build
// line in the package doc comment, or next to the package clause without a
// blank line after it, is an ordinary comment and is not read.
//
// The rule reads the files the current build includes. A file the build
// excludes is not offered to the analyzer, so a duplicate there waits for a
// run on a platform that includes it. A non-Go file is not read, even though
// the go command reads its constraints.
//
// Two conditions are the same when they differ only in the order of terms
// joined by && or ||.
//
// The fix removes the repeated line.
//
// Adapted from staticcheck's SA4019.
package dupbuildtag

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/token"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/buildtag"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupbuildtag",
	Doc:  "reports a // +build line whose condition an earlier line gives",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		check(pass, file)
	}
	return nil, nil
}

// check reports each // +build line whose condition an earlier line in file
// already gives.
func check(pass *analysis.Pass, file *ast.File) {
	cutoff, ok := buildtag.HeaderEnd(pass, file)
	if !ok {
		return
	}
	seen := make(map[string]string)
	for _, group := range file.Comments {
		if group.Pos() >= cutoff {
			break
		}
		for _, comment := range group.List {
			if !constraint.IsPlusBuild(comment.Text) || !valid(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				continue
			}
			text := condition(comment.Text)
			key := canonical(expr)
			if first, ok := seen[key]; ok {
				report(pass, file, comment, first, text)
				continue
			}
			seen[key] = text
		}
	}
}

// valid reports whether every tag in a legacy line can be read. The parser
// turns an invalid tag into ignore, so two unrelated malformed lines would
// otherwise compare as the same condition.
func valid(text string) bool {
	text = condition(text)
	if text == "" {
		return false
	}
	for clause := range strings.FieldsSeq(text) {
		for literal := range strings.SplitSeq(clause, ",") {
			if strings.HasPrefix(literal, "!!") || literal == "!" {
				return false
			}
			if !validTag(strings.TrimPrefix(literal, "!")) {
				return false
			}
		}
	}
	return true
}

// validTag reports whether tag is a legacy build tag: letters, digits,
// underscores or dots.
func validTag(tag string) bool {
	if tag == "" {
		return false
	}
	for _, r := range tag {
		if !validTagRune(r) {
			return false
		}
	}
	return true
}

// validTagRune reports whether r may appear in a legacy build tag.
func validTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}

// condition returns the text after the // +build prefix.
func condition(text string) string {
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "+build")
	return strings.TrimSpace(text)
}

// canonical returns a form of expr that is the same for two conditions that
// differ only in the order of terms joined by && or ||.
func canonical(expr constraint.Expr) string {
	switch expr := expr.(type) {
	case *constraint.TagExpr:
		return expr.Tag
	case *constraint.NotExpr:
		return "!" + canonical(expr.X)
	case *constraint.AndExpr:
		terms := andTerms(expr)
		slices.Sort(terms)
		return "(" + strings.Join(terms, " && ") + ")"
	case *constraint.OrExpr:
		terms := orTerms(expr)
		slices.Sort(terms)
		return "(" + strings.Join(terms, " || ") + ")"
	}
	return ""
}

// andTerms returns the operands of a run of && in expr.
func andTerms(expr constraint.Expr) []string {
	if and, ok := expr.(*constraint.AndExpr); ok {
		return append(andTerms(and.X), andTerms(and.Y)...)
	}
	return []string{canonical(expr)}
}

// orTerms returns the operands of a run of || in expr.
func orTerms(expr constraint.Expr) []string {
	if or, ok := expr.(*constraint.OrExpr); ok {
		return append(orTerms(or.X), orTerms(or.Y)...)
	}
	return []string{canonical(expr)}
}

// report reports the repeated line and offers the edit that removes it.
func report(pass *analysis.Pass, file *ast.File, duplicate *ast.Comment, first, second string) {
	diagnostic := analysis.Diagnostic{
		Pos: file.Package,
		Message: fmt.Sprintf(
			"equivalent build constraints %q and %q",
			first, second,
		),
	}
	if edit, ok := removal(pass, duplicate); ok {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Remove the repeated line",
			TextEdits: []analysis.TextEdit{edit},
		}}
	}
	pass.Report(diagnostic)
}

// removal returns the edit that deletes the line holding comment. It fails
// when the file on disk is not the file that was parsed, since then the edit
// would remove different bytes.
func removal(pass *analysis.Pass, comment *ast.Comment) (analysis.TextEdit, bool) {
	tokFile := pass.Fset.File(comment.Pos())
	if tokFile == nil {
		return analysis.TextEdit{}, false
	}
	src, err := pass.ReadFile(tokFile.Name())
	if err != nil {
		return analysis.TextEdit{}, false
	}
	if !holds(tokFile, src, comment) {
		return analysis.TextEdit{}, false
	}
	line := tokFile.PositionFor(comment.Pos(), false).Line
	start := tokFile.LineStart(line)
	before := src[tokFile.Offset(start):tokFile.Offset(comment.Pos())]
	if strings.TrimSpace(string(before)) != "" {
		return analysis.TextEdit{}, false
	}
	if line < tokFile.LineCount() {
		return analysis.TextEdit{
			Pos: start,
			End: tokFile.LineStart(line + 1),
		}, true
	}
	return analysis.TextEdit{
		Pos: start,
		End: tokFile.Pos(tokFile.Size()),
	}, true
}

// holds reports whether src still holds comment at the offset where it was
// parsed.
func holds(tokFile *token.File, src []byte, comment *ast.Comment) bool {
	lo, hi := tokFile.Offset(comment.Pos()), tokFile.Offset(comment.End())
	if lo < 0 || hi > len(src) || hi <= lo {
		return false
	}
	return string(src[lo:hi]) == comment.Text
}
