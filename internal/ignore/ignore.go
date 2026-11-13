package ignore

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Apply returns copies of analyzers that drop the diagnostics an //ignore
// directive covers, and every diagnostic inside a generated file.
//
// A generated file is not the author's to fix, so every analyzer in the suite
// and every standalone command leaves one alone. The file is still read: a
// rule that reasons about the package needs every declaration in it, and
// hiding the syntax would make that reasoning unsound. Only the report is
// suppressed. Keeping the rule here rather than in each analyzer keeps a
// command and the suite agreeing about what a directive and a generated file
// mean.
//
// The copies keep their names, flags, facts and prerequisites, so a driver
// sees the suite as it was written: one analyzer per linter.
func Apply(analyzers ...*analysis.Analyzer) []*analysis.Analyzer {
	filtered := make([]*analysis.Analyzer, 0, len(analyzers))
	for _, a := range analyzers {
		filtered = append(filtered, filterAnalyzer(a))
	}
	return filtered
}

type ignoreRange struct {
	start, end token.Pos
	analyzers  map[string]struct{}
}

// ignore computes the ranges that //ignore directives cover. Every
// filtered analyzer requires it, so it runs once per package.
var ignore = &analysis.Analyzer{
	Name:       "vetignore",
	Doc:        "computes the ranges that //ignore directives cover",
	ResultType: reflect.TypeFor[[]ignoreRange](),
	Run: func(pass *analysis.Pass) (any, error) {
		return ignoreRanges(pass), nil
	},
}

// filterAnalyzer returns a copy of a that drops diagnostics inside the
// ranges an //ignore directive covers for a.
func filterAnalyzer(a *analysis.Analyzer) *analysis.Analyzer {
	filtered := *a
	filtered.Requires = append(slices.Clone(a.Requires), ignore)
	run := a.Run
	filtered.Run = func(pass *analysis.Pass) (any, error) {
		ranges, ok := pass.ResultOf[ignore].([]ignoreRange)
		if !ok {
			return nil, fmt.Errorf("%s: missing ignore ranges", a.Name)
		}
		inner := *pass
		inner.Analyzer = a
		inner.Report = func(d analysis.Diagnostic) {
			if !ignoreDiagnostic(d, ranges, a.Name, pass.Fset) {
				pass.Report(d)
			}
		}
		return run(&inner)
	}
	return &filtered
}

func ignoreRanges(pass *analysis.Pass) (ranges []ignoreRange) {
	for _, file := range pass.Files {
		ranges = append(ranges, fileIgnores(file, pass)...)
	}
	return
}

func fileIgnores(file *ast.File, pass *analysis.Pass) (ranges []ignoreRange) {
	if ast.IsGenerated(file) {
		return []ignoreRange{{
			start:     file.FileStart,
			end:       file.End(),
			analyzers: map[string]struct{}{"all": {}},
		}}
	}
	cmap := ast.NewCommentMap(pass.Fset, file, file.Comments)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			analyzers, in := parseIgnore(c.Text)
			if analyzers == nil {
				continue
			}
			if in {
				ranges = append(ranges, ignoreCommentLine(c, pass, analyzers))
			} else {
				ranges = append(
					ranges,
					newIgnoreRange(c, analyzers, file, cmap, cg, pass),
				)
			}
		}
	}
	return
}

func newIgnoreRange(comment *ast.Comment, analyzers map[string]struct{}, file *ast.File, cmap ast.CommentMap, group *ast.CommentGroup, pass *analysis.Pass) ignoreRange {
	if comment.Pos() < file.Package {
		// Ignore analyzers on this entire file.
		return ignoreRange{file.FileStart, file.End(), analyzers}
	}
	node := findCommentNode(comment, cmap)
	switch {
	case node != nil && group != nil:
		if isInlineComment(comment, pass) {
			// Ignore this comment's associated line.
			return ignoreCommentLine(comment, pass, analyzers)
		}
		// Ignore analyzers on this comment group and its associated node.
		return ignoreRange{
			lineStart(pass, min(group.Pos(), node.Pos())),
			lineEnd(pass, max(group.End(), node.End())), analyzers,
		}
	case node != nil:
		// Ignore analyzers on this node.
		return ignoreRange{
			lineStart(pass, node.Pos()),
			lineEnd(pass, node.End()), analyzers,
		}
	case group != nil:
		// Ignore analyzers on this comment group.
		return ignoreRange{
			lineStart(pass, group.Pos()),
			lineEnd(pass, group.End()), analyzers,
		}
	}
	// Ignore analyzers on the current line.
	return ignoreCommentLine(comment, pass, analyzers)
}

var commentRe = regexp.MustCompile(`^\s*\/$`)

func isInlineComment(comment *ast.Comment, pass *analysis.Pass) bool {
	file := pass.Fset.File(comment.Pos())
	buf, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	pos := file.Offset(file.LineStart(pass.Fset.Position(comment.Pos()).Line))
	end := file.Offset(comment.Pos() + 1)
	return !commentRe.Match(buf[pos:end])
}

func ignoreCommentLine(comment *ast.Comment, pass *analysis.Pass, analyzers map[string]struct{}) ignoreRange {
	start, end := lineStart(pass, comment.Pos()), lineEnd(pass, comment.Pos())
	return ignoreRange{start, end, analyzers}
}

// lineStart widens p to the start of its line, since a diagnostic may
// anchor at column 1.
func lineStart(pass *analysis.Pass, p token.Pos) token.Pos {
	file := pass.Fset.File(p)
	if file == nil {
		return p
	}
	return file.LineStart(file.Line(p))
}

// lineEnd widens p to the end of its line, since a diagnostic may
// anchor at the line's last character.
func lineEnd(pass *analysis.Pass, p token.Pos) token.Pos {
	file := pass.Fset.File(p)
	if file == nil {
		return p
	}
	line := file.Line(p)
	if line < file.LineCount() {
		return file.LineStart(line+1) - 1
	}
	return token.Pos(file.Base() + file.Size())
}

func findCommentNode(comment *ast.Comment, cmap ast.CommentMap) ast.Node {
	for node, commentGroups := range cmap {
		for _, cg := range commentGroups {
			if slices.Contains(cg.List, comment) {
				return node
			}
		}
	}
	return nil
}

func ignoreDiagnostic(diag analysis.Diagnostic, ranges []ignoreRange, analyzerName string, fset *token.FileSet) bool {
	if !diag.Pos.IsValid() {
		return false
	}
	diagPos := fset.Position(diag.Pos)

	for _, r := range ranges {
		if !r.start.IsValid() {
			continue
		}
		if fset.Position(r.start).Filename != diagPos.Filename {
			continue
		}
		if analyzersContains(r.analyzers, analyzerName) {
			if diag.Pos >= r.start && diag.Pos <= r.end {
				return true
			}
		}
	}
	return false
}

func analyzersContains(analyzers map[string]struct{}, analyzerName string) bool {
	if _, ok := analyzers["all"]; ok {
		return true
	}
	_, exists := analyzers[analyzerName]
	return exists
}

var ignoreRe = regexp.MustCompile(`//ignore(?::([^/\s]+))?`)

func parseIgnore(text string) (analyzers map[string]struct{}, in bool) {
	match := ignoreRe.FindStringSubmatch(text)
	if match == nil {
		return
	}
	in = !strings.HasPrefix(text, match[0])
	analyzers = make(map[string]struct{})
	if match[1] == "" {
		analyzers["all"] = struct{}{}
		return
	}
	analyzerList := match[1]
	if analyzerList == "all" {
		analyzers["all"] = struct{}{}
		return
	}
	for name := range strings.SplitSeq(analyzerList, ",") {
		if name = strings.TrimSpace(name); name != "" {
			analyzers[name] = struct{}{}
		}
	}
	return
}
