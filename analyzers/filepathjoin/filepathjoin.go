// Package filepathjoin provides an analysis.Analyzer that reports a path
// concatenated with a literal separator, as dir + "/" + name does.
//
// A slash in a string is more often part of a URL, a test name, or an archive
// member than a filesystem path, and nothing in the expression says which. The
// file's import of path/filepath is what says which, and it is also what lets
// the fix be written: a package that path/filepath depends on cannot import it
// back. So the rule reports in a file that has the import, and nowhere else.
//
// The fix replaces the path with filepath.Join of its elements. The rewrite is
// the cleaned path, and the rule treats that as the path the code meant, so a
// separator an element ends with, an element that is "..", and a value that is
// empty at run time all produce a different string afterwards. A path written
// across lines, an expression holding a comment, and a rewrite that would run
// past its line report without a fix.
//
// The README catalogs every check, exemption, and fix. The rule is adapted
// from go-critic's preferFilepathJoin.
package filepathjoin

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "filepathjoin",
	Doc:  "reports a path concatenated with a literal separator",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			defer func() { stack = append(stack, n) }()
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || bin.Op != token.ADD || chainAbove(stack) {
				return true
			}
			check(pass, file, bin)
			return true
		})
	}
	return nil, nil
}

// chainAbove reports whether a + expression encloses bin, which that
// expression already covers.
func chainAbove(stack []ast.Node) bool {
	for _, n := range slices.Backward(stack) {
		switch n := n.(type) {
		case *ast.ParenExpr:
			continue
		case *ast.BinaryExpr:
			return n.Op == token.ADD
		default:
			return false
		}
	}
	return false
}

// check reports the path in bin, which is one chain of + expressions.
func check(pass *analysis.Pass, file *ast.File, bin *ast.BinaryExpr) {
	// A chain of constants is one literal written in pieces, which
	// uselessconcat owns.
	if pass.TypesInfo.Types[bin].Value != nil {
		return
	}
	operands := flatten(bin)
	if slices.ContainsFunc(operands, foreign) {
		return
	}
	// Text glued to an element would lose the separator that element ends
	// with, because filepath.Join Cleans it away, so the path is the part
	// that ends the chain.
	last := len(operands) - 1
	if separator(operands[last]) {
		return
	}
	first := last
	for first >= 2 && alternates(operands, first-2) {
		first -= 2
	}
	if first == last {
		return
	}
	report(pass, file, operands[first:last+1])
}

// alternates reports whether the element at operands[i] is followed by a
// separator and then another element.
func alternates(operands []ast.Expr, i int) bool {
	return !separator(operands[i]) && separator(operands[i+1]) &&
		!separator(operands[i+2])
}

// flatten returns the operands of a + chain in source order.
func flatten(bin *ast.BinaryExpr) (out []ast.Expr) {
	for _, e := range []ast.Expr{bin.X, bin.Y} {
		if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
			out = append(out, flatten(b)...)
			continue
		}
		out = append(out, e)
	}
	return out
}

// report reports one path, which alternates elements with separator literals.
func report(pass *analysis.Pass, file *ast.File, path []ast.Expr) {
	var (
		first, last = path[0], path[len(path)-1]
		name, ok    = joinName(pass, file, first.Pos())
	)
	if !ok {
		return
	}
	var elements []ast.Expr
	for _, e := range path {
		if separator(e) {
			continue
		}
		// A bracketed path is part of the expression around it, and
		// this rewrite would have to reach across the brackets. A
		// filepath.Join with no arguments reads as an empty element,
		// which the rewrite would drop.
		plain := ast.Unparen(e)
		if bracketed(plain) || emptyJoin(pass, plain) {
			return
		}
		if !plainString(pass, plain) {
			return
		}
		elements = append(elements, plain)
	}
	d := analysis.Diagnostic{
		Pos: first.Pos(),
		End: last.End(),
		Message: "path concatenated with a literal separator can be replaced by " +
			name,
	}
	// A comment inside the expression would go with the rewrite, and a
	// rewrite that runs past its line is not worth making; either leaves
	// the report without a fix.
	text := name + "(" + arguments(pass, elements) + ")"
	fixable := !commented(file, first.Pos(), last.End())
	if fixable && fits(pass, first.Pos(), last.End(), text) {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace the concatenation with " + name,
			TextEdits: []analysis.TextEdit{{
				Pos:     first.Pos(),
				End:     last.End(),
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(d)
}

// bracketed reports whether e is a parenthesized path, which the chain around
// it would have to reach across. Brackets around text that is not a path are
// one argument, and the rewrite prints them as one.
func bracketed(e ast.Expr) bool {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.ADD {
		return false
	}
	return slices.ContainsFunc(flatten(bin), separator)
}

// separator reports whether e is a string literal holding exactly one byte
// that os.IsPathSeparator accepts, so a backslash separates on Windows and is
// an ordinary character on Unix.
func separator(e ast.Expr) bool {
	s, ok := lit(e)
	return ok && len(s) == 1 && os.IsPathSeparator(s[0])
}

// foreign reports whether e is a literal that says the chain is not a plain
// path: a URL, a format string, a glob, or an empty string, whose separator
// the join drops.
func foreign(e ast.Expr) bool {
	s, ok := lit(e)
	if !ok {
		return false
	}
	if s == "" {
		return true
	}
	return strings.Contains(s, "://") || strings.ContainsRune(s, '%') ||
		strings.ContainsAny(s, "*?[")
}

// emptyJoin reports whether e is a filepath.Join call with no arguments.
func emptyJoin(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && len(call.Args) == 0 && join(pass, call)
}

// lit returns the value of e's string literal, and whether e is one.
func lit(e ast.Expr) (string, bool) {
	basic, ok := ast.Unparen(e).(*ast.BasicLit)
	if !ok || basic.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(basic.Value)
	return s, err == nil
}

// plainString reports whether e has type string itself, which is the type
// filepath.Join takes. A named string type would need a conversion, and a type
// parameter is not string.
func plainString(pass *analysis.Pass, e ast.Expr) bool {
	basic, ok := types.Unalias(pass.TypesInfo.TypeOf(e)).(*types.Basic)
	return ok && basic.Info()&types.IsString != 0
}

// arguments renders the elements as the argument list of a call. An element
// that is a filepath.Join call contributes its own arguments: the two calls
// build the same path, and the shorter one is the one to write.
func arguments(pass *analysis.Pass, elements []ast.Expr) string {
	var text []string
	for _, e := range elements {
		for _, arg := range joined(pass, e) {
			text = append(text, expr(arg))
		}
	}
	return strings.Join(text, ", ")
}

// expr prints e on one line. A fresh FileSet drops the source line breaks, and
// go/printer keeps the elements of a composite literal, which types.ExprString
// would elide into text that does not parse.
func expr(e ast.Expr) string {
	var buf bytes.Buffer
	// A bytes.Buffer cannot fail.
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
}

// joined returns the arguments of a filepath.Join call, and the expression
// itself when it is something else. A call that spreads a slice keeps its own
// argument list: the spread belongs in last position, and the rewrite would
// put it after the call's other arguments.
func joined(pass *analysis.Pass, e ast.Expr) []ast.Expr {
	call, ok := e.(*ast.CallExpr)
	if !ok || call.Ellipsis.IsValid() || !join(pass, call) {
		return []ast.Expr{e}
	}
	return call.Args
}

// join reports whether call is a call of path/filepath.Join.
func join(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	return fn != nil && fn.FullName() == "path/filepath.Join"
}

// joinName returns the text that names filepath.Join in file, and whether the
// fix can write it.
//
// The name is the one the file's import of path/filepath gives. A file without
// that import is left alone: path/filepath imports os and io/fs, so the
// package holding the code may be one it cannot import. A dot import gives the
// package no qualifier, and a name that something else in scope holds would
// send the call there.
func joinName(pass *analysis.Pass, file *ast.File, pos token.Pos) (string, bool) {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "path/filepath" {
			continue
		}
		name := "filepath"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "_" || name == "." || shadowed(pass, file, pos, name) {
			return "", false
		}
		return name + ".Join", true
	}
	return "", false
}

// shadowed reports whether name resolves to anything other than an import at
// pos, which would make the identifier the fix writes mean that instead.
func shadowed(pass *analysis.Pass, file *ast.File, pos token.Pos, name string) bool {
	scope := pass.TypesInfo.Scopes[file]
	if scope == nil {
		return true
	}
	if scope = scope.Innermost(pos); scope == nil {
		return true
	}
	_, obj := scope.LookupParent(name, pos)
	if obj == nil {
		return true
	}
	pkg, ok := obj.(*types.PkgName)
	return !ok || pkg.Imported().Path() != "path/filepath"
}

// fits reports whether text may replace the range from to. The range lies on
// one line, and the rewritten line is within the width unless the line is
// already past it.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	// Written positions, which a //line directive does not move.
	start, end := pass.Fset.PositionFor(from, false),
		pass.Fset.PositionFor(to, false)
	if start.Line != end.Line {
		return false
	}
	if linelen.Config.Len <= 0 {
		return true
	}
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	line := lines[start.Line-1]
	if start.Column > len(line)+1 || end.Column > len(line)+1 {
		return false
	}
	if !linelen.CheckLine(line) {
		return true
	}
	var rewritten bytes.Buffer
	rewritten.Write(line[:start.Column-1])
	rewritten.WriteString(text)
	rewritten.Write(line[end.Column-1:])
	return linelen.CheckLine(rewritten.Bytes())
}

// commented reports whether a comment sits inside the span a fix would
// replace, where it would be dropped with the expression.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if c.Pos() >= lo && c.End() <= hi {
				return true
			}
		}
	}
	return false
}
