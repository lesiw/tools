// Package dotimport provides an analysis.Analyzer that reports a dot import in
// a file that is not a test.
//
// The spec's import declarations section says: "all the package's exported
// identifiers declared in that package's package block will be declared in
// the importing source file's file block and must be accessed without a
// qualifier." A reader who sees Contains in such a file cannot tell whether it
// comes from the file, its package, or an imported package.
//
// The Code Review Comments' "Import Dot" allows the form in a test that cannot
// be part of the package it tests: "Except for this one case, do not use
// import . in your programs." Every test file is exempt, which is wider than
// the cited case.
//
// The suggested fix removes the dot and qualifies every use of a package-level
// identifier with the name the file binds to the path, using an existing
// import or the package's own name. It is offered when that name reaches the
// imported package at every use. An in-place conversion is not offered when a
// second dot-imported package has the same name, or when the dot and path are
// on different lines. Removing a dot import is not offered when its
// declaration carries a doc comment or the import shares a line with another
// import. The fix is also not offered when a qualifier or a moved comment
// would push a line past the width, or when a comment sits between the dot and
// the path. A single-line trailing comment moves to the import that stays when
// that import has no comment; any other trailing comment blocks the fix.
//
// A dot import of internal/types/errors is left alone: go/types and
// cmd/compile/internal/types2 use that package for their error codes. The
// -dotimport.allowed flag adds import paths to the default list.
//
// Inspired by revive's dot-imports.
package dotimport

import (
	"bytes"
	"flag"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/imports"
)

// allowed contains the import paths that are not reported.
var allowed = map[string]struct{}{
	"internal/types/errors": {},
}

// Flags registers the import paths that are not reported.
func Flags(fs *flag.FlagSet) {
	fs.Func("allowed", "comma-separated import paths that are not reported", addAllowed)
}

func addAllowed(paths string) error {
	for path := range strings.SplitSeq(paths, ",") {
		path = strings.TrimSpace(path)
		if path != "" {
			allowed[path] = struct{}{}
		}
	}
	return nil
}

var Analyzer = &analysis.Analyzer{
	Name: "dotimport",
	Doc:  "reports a dot import in a file that is not a test",
	Run:  run,
}

func init() { Flags(&Analyzer.Flags) }

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		name := pass.Fset.File(file.FileStart).Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, spec := range file.Imports {
			if spec.Name == nil || spec.Name.Name != "." {
				continue
			}
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || exempt(path) {
				continue
			}
			diag := analysis.Diagnostic{
				Pos:     spec.Path.Pos(),
				Message: "dot import of " + strconv.Quote(path),
			}
			if edits, ok := qualify(pass, file, spec, path); ok {
				diag.SuggestedFixes = []analysis.SuggestedFix{{
					Message:   "Qualify the names with the package name",
					TextEdits: edits,
				}}
			}
			pass.Report(diag)
		}
	}
	return nil, nil
}

// exempt reports whether path is in the allow list.
func exempt(path string) bool {
	_, ok := allowed[path]
	return ok
}

// qualify returns the edits that remove the dot from spec and prefix every use
// of a name from path with the name the file binds to it. It returns false
// when the rewrite is not a plain qualification.
func qualify(pass *analysis.Pass, file *ast.File, spec *ast.ImportSpec, path string) ([]analysis.TextEdit, bool) {
	if commentBetween(file, spec) {
		return nil, false
	}
	pkg := imported(pass, path)
	if pkg == nil || pkg.Name() == "" {
		return nil, false
	}
	qualifier, dropSpec, ok := qualification(file, spec, path, pkg.Name())
	if !ok {
		return nil, false
	}
	if !dropSpec && !sameLine(pass, spec) {
		return nil, false
	}
	names := imports.Names(pass, file)
	delete(names, path)
	if !dropSpec && sameNameDot(pass, file, spec, path, qualifier) {
		return nil, false
	}
	names[path] = qualifier
	edits, ok := useEdits(pass, file, path, names)
	if !ok {
		return nil, false
	}
	return rewrite(pass, file, spec, path, dropSpec, edits)
}

// rewrite adds the import edit and reports whether the result is safe.
func rewrite(pass *analysis.Pass, file *ast.File, spec *ast.ImportSpec, path string, dropSpec bool, edits []analysis.TextEdit) ([]analysis.TextEdit, bool) {
	if !fixable(dropSpec, spec, edits) {
		return nil, false
	}
	if !dropSpec {
		edits = append(edits, analysis.TextEdit{
			Pos: spec.Name.Pos(),
			End: spec.Path.Pos(),
		})
		if !fits(pass, edits) {
			return nil, false
		}
		return edits, true
	}
	if !movableComment(pass, file, spec, path) {
		return nil, false
	}
	start, end, ok := removalSpan(pass, file, spec)
	if !ok {
		return nil, false
	}
	edits = append(edits, analysis.TextEdit{Pos: start, End: end})
	if spec.Comment != nil {
		edits = append(edits, commentEdit(file, spec, path))
	}
	if !fits(pass, edits) {
		return nil, false
	}
	return edits, true
}

// fixable reports whether the spec's rewrite has a safe form.
func fixable(dropSpec bool, spec *ast.ImportSpec, edits []analysis.TextEdit) bool {
	if dropSpec {
		return spec.Doc == nil
	}
	return len(edits) > 0
}

// useEdits returns the qualifier insertions for every use of path in file.
func useEdits(pass *analysis.Pass, file *ast.File, path string, names map[string]string) (edits []analysis.TextEdit, ok bool) {
	ok = true
	selected := make(map[*ast.Ident]struct{})
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			selected[sel.Sel] = struct{}{}
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		id, found := n.(*ast.Ident)
		if !found {
			return true
		}
		if _, found := selected[id]; found {
			return true
		}
		obj := pass.TypesInfo.Uses[id]
		if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != path {
			return true
		}
		if isField(obj) {
			return true
		}
		name, found := imports.Qualifier(pass, names, path, id.Pos())
		if !found {
			ok = false
			return false
		}
		edits = append(edits, analysis.TextEdit{
			Pos:     id.Pos(),
			End:     id.Pos(),
			NewText: []byte(name + "."),
		})
		return true
	})
	return edits, ok
}

// commentEdit returns the edit that writes spec's trailing comment at the
// import for path that stays.
func commentEdit(file *ast.File, spec *ast.ImportSpec, path string) analysis.TextEdit {
	existing, _ := otherImport(file, spec, path)
	return analysis.TextEdit{
		Pos:     importEnd(existing),
		End:     importEnd(existing),
		NewText: []byte(" " + commentText(spec.Comment)),
	}
}

// importEnd returns the end of spec's path or its trailing comment.
func importEnd(spec *ast.ImportSpec) token.Pos {
	if spec.Comment != nil {
		return spec.Comment.End()
	}
	return spec.End()
}

// sameNameDot reports whether another dot-imported package has the same name,
// which would collide with the name an in-place conversion writes.
func sameNameDot(pass *analysis.Pass, file *ast.File, spec *ast.ImportSpec, path, qualifier string) bool {
	for _, other := range file.Imports {
		if other == spec || other.Name == nil || other.Name.Name != "." {
			continue
		}
		otherPath, err := strconv.Unquote(other.Path.Value)
		if err != nil || otherPath == path {
			continue
		}
		pkg := imported(pass, otherPath)
		if pkg != nil && pkg.Name() == qualifier {
			return true
		}
	}
	return false
}

// insertion is one qualifier written at a source position.
type insertion struct {
	line int
	col  int
	text string
}

// fits reports whether the qualifier insertions keep their lines within the
// width.
func fits(pass *analysis.Pass, edits []analysis.TextEdit) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	var (
		file string
		list []insertion
	)
	for _, edit := range edits {
		start := pass.Fset.PositionFor(edit.Pos, false)
		if start.Filename == "" || start.Line == 0 || len(edit.NewText) == 0 {
			continue
		}
		if file == "" {
			file = start.Filename
		}
		if start.Filename != file {
			return false
		}
		list = append(list, insertion{
			line: start.Line,
			col:  start.Column,
			text: string(edit.NewText),
		})
	}
	if len(list) == 0 {
		return true
	}
	src, err := pass.ReadFile(file)
	if err != nil {
		return false
	}
	lines := bytes.Split(src, []byte("\n"))
	slices.SortFunc(list, func(a, b insertion) int { return a.line - b.line })
	for i := 0; i < len(list); {
		line, j := list[i].line, i
		for j < len(list) && list[j].line == line {
			j++
		}
		if line > len(lines) {
			return false
		}
		if !rewrittenFits(lines[line-1], list[i:j]) {
			return false
		}
		i = j
	}
	return true
}

// rewrittenFits reports whether line with insertions stays within the width.
func rewrittenFits(line []byte, list []insertion) bool {
	if !linelen.CheckLine(line) {
		return true
	}
	slices.SortFunc(list, func(a, b insertion) int { return a.col - b.col })
	for _, ins := range list {
		if ins.col < 1 || ins.col-1 > len(line) {
			return false
		}
	}
	out := make([]byte, 0, len(line)+len(list)*8)
	out = append(out, line[:list[0].col-1]...)
	out = append(out, list[0].text...)
	for i := 1; i < len(list); i++ {
		out = append(out, line[list[i-1].col-1:list[i].col-1]...)
		out = append(out, list[i].text...)
	}
	out = append(out, line[list[len(list)-1].col-1:]...)
	return linelen.CheckLine(out)
}

// commentText returns the text of group with its markers, joined by spaces.
func commentText(group *ast.CommentGroup) string {
	var text strings.Builder
	for _, comment := range group.List {
		if text.Len() > 0 {
			text.WriteByte(' ')
		}
		text.WriteString(comment.Text)
	}
	return text.String()
}

// qualification returns the name to write for path and whether the dot spec
// goes away. It returns false when the rewrite has no plain form.
func qualification(file *ast.File, spec *ast.ImportSpec, path, pkgName string) (qualifier string, dropSpec, ok bool) {
	existing, duplicate := otherImport(file, spec, path)
	if duplicate {
		return "", false, false
	}
	if existing == nil {
		return pkgName, false, true
	}
	switch {
	case existing.Name == nil:
		return pkgName, true, true
	case existing.Name.Name == "_":
		return pkgName, false, true
	case existing.Name.Name == ".":
		return "", false, false
	default:
		return existing.Name.Name, true, true
	}
}

// otherImport returns the import in file for path. It reports a duplicate dot
// import separately, since that file has two bindings of one path.
func otherImport(file *ast.File, spec *ast.ImportSpec, path string) (*ast.ImportSpec, bool) {
	var blank *ast.ImportSpec
	for _, other := range file.Imports {
		if other == spec {
			continue
		}
		got, err := strconv.Unquote(other.Path.Value)
		if err != nil || got != path {
			continue
		}
		switch {
		case other.Name == nil:
			return other, false
		case other.Name.Name == ".":
			return nil, true
		case other.Name.Name == "_":
			blank = other
		default:
			return other, false
		}
	}
	return blank, false
}

// sameLine reports whether the dot and the path sit on one line.
func sameLine(pass *analysis.Pass, spec *ast.ImportSpec) bool {
	return pass.Fset.PositionFor(spec.Name.Pos(), false).Line ==
		pass.Fset.PositionFor(spec.Path.Pos(), false).Line
}

// movableComment reports whether spec's trailing comment can move to the
// import for path that stays.
func movableComment(pass *analysis.Pass, file *ast.File, spec *ast.ImportSpec, path string) bool {
	if spec.Comment == nil {
		return true
	}
	if len(spec.Comment.List) != 1 {
		return false
	}
	comment := spec.Comment.List[0]
	if !lineComment(comment.Text) {
		return false
	}
	existing, _ := otherImport(file, spec, path)
	if existing == nil || existing.Comment != nil {
		return false
	}
	return aloneImport(pass, file, existing)
}

// aloneImport reports whether spec's import declaration is alone on its line.
func aloneImport(pass *analysis.Pass, file *ast.File, spec *ast.ImportSpec) bool {
	decl := importDecl(file, spec)
	if decl != nil && !decl.Lparen.IsValid() {
		return aloneOnLine(pass, decl)
	}
	return aloneOnLine(pass, spec)
}

// aloneOnLine reports whether node is the only content on its line.
func aloneOnLine(pass *analysis.Pass, node ast.Node) bool {
	file := pass.Fset.File(node.Pos())
	if file == nil {
		return false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	start := pass.Fset.PositionFor(node.Pos(), false)
	end := pass.Fset.PositionFor(node.End(), false)
	lines := bytes.Split(src, []byte("\n"))
	if start.Line != end.Line || start.Line > len(lines) {
		return false
	}
	line := lines[start.Line-1]
	inside := start.Column >= 1 && end.Column >= 1 &&
		start.Column <= len(line)+1 && end.Column <= len(line)+1
	if !inside {
		return false
	}
	return len(bytes.TrimSpace(line[:start.Column-1])) == 0 &&
		len(bytes.TrimSpace(line[end.Column-1:])) == 0
}

// lineComment reports whether text is one line comment.
func lineComment(text string) bool {
	return strings.HasPrefix(text, "//") && !strings.Contains(text, "\n")
}

// removalSpan returns the span to remove for spec. A declaration that holds
// only spec goes as a whole; otherwise spec and its trailing comment go.
func removalSpan(pass *analysis.Pass, file *ast.File, spec *ast.ImportSpec) (start, end token.Pos, ok bool) {
	decl := importDecl(file, spec)
	if decl != nil && len(decl.Specs) == 1 {
		if decl.Doc != nil {
			return 0, 0, false
		}
		if decl.Lparen.IsValid() {
			return lineSpan(pass, decl, nil)
		}
		return lineSpan(pass, decl, spec.Comment)
	}
	return lineSpan(pass, spec, spec.Comment)
}

// importDecl returns the import declaration that holds spec.
func importDecl(file *ast.File, spec *ast.ImportSpec) *ast.GenDecl {
	for _, node := range file.Decls {
		decl, ok := node.(*ast.GenDecl)
		if !ok || decl.Tok != token.IMPORT {
			continue
		}
		for _, item := range decl.Specs {
			if item == spec {
				return decl
			}
		}
	}
	return nil
}

// lineSpan returns the span to remove for node and its trailing comment. It
// reports false when the node is not alone on its lines, since removing part
// of a line would leave invalid syntax.
func lineSpan(pass *analysis.Pass, node ast.Node, comment *ast.CommentGroup) (start, end token.Pos, ok bool) {
	file := pass.Fset.File(node.Pos())
	if file == nil {
		return 0, 0, false
	}
	start = node.Pos()
	end = node.End()
	if comment != nil {
		end = comment.End()
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return 0, 0, false
	}
	lines := bytes.Split(src, []byte("\n"))
	startLine := file.Line(start)
	endLine := file.Line(end)
	if startLine > len(lines) || endLine > len(lines) {
		return 0, 0, false
	}
	startCol := pass.Fset.PositionFor(start, false).Column
	endCol := pass.Fset.PositionFor(end, false).Column
	if startCol < 1 || endCol < 1 {
		return 0, 0, false
	}
	before := lines[startLine-1][:startCol-1]
	after := lines[endLine-1][endCol-1:]
	if len(bytes.TrimSpace(before)) > 0 || len(bytes.TrimSpace(after)) > 0 {
		return 0, 0, false
	}
	start = file.LineStart(startLine)
	if endLine < file.LineCount() {
		end = file.LineStart(endLine + 1)
	}
	return start, end, true
}

// commentBetween reports whether a comment sits between the dot and the path.
func commentBetween(file *ast.File, spec *ast.ImportSpec) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if between(comment, spec) {
				return true
			}
		}
	}
	return false
}

// between reports whether comment sits between the dot and the path.
func between(comment *ast.Comment, spec *ast.ImportSpec) bool {
	return comment.Pos() >= spec.Name.End() && comment.End() <= spec.Path.Pos()
}

// imported returns the package imported at path.
func imported(pass *analysis.Pass, path string) *types.Package {
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() == path {
			return pkg
		}
	}
	return nil
}

// isField reports whether obj is a field, which is selected through a value or
// written as a field key, so a package qualifier would be wrong. A method use
// is a selector name, which selected holds.
func isField(obj types.Object) bool {
	field, ok := obj.(*types.Var)
	return ok && field.IsField()
}
