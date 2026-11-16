// Package predictivetemp provides an analysis.Analyzer that reports a file
// created at a fixed path in a shared directory, as os.Create("/tmp/report")
// does.
//
// A fixed name in a shared directory is one file for every process on the
// machine, and every user can write there, so another process may already have
// created it. os.CreateTemp's doc promises that programs calling it
// simultaneously "will not choose the same file". A fixed name offers no such
// guarantee.
//
// The fix writes os.CreateTemp with the path's directory and its file name as
// the pattern. The fixed name is the fault, so the repair changes which file
// the program opens: a new one on every run, in mode 0o600, that nothing
// removes. It is offered only where the call keeps the file and no other
// constant in the package names the path, so the name the rewrite picks is the
// one the program has to work with. An os.OpenFile is reported without a fix:
// os.CreateTemp takes no flag and no mode, and both would be dropped.
//
// The rule is adapted from gosec's G303, which matches a path prefix under
// /usr/tmp, /var/tmp, or /tmp, covers os.WriteFile as well, follows a local
// variable to its initializer, and offers no fix. The README catalogs every
// check, exemption, and fix.
package predictivetemp

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"lesiw.io/tools/analyzers/linelen"
)

var Analyzer = &analysis.Analyzer{
	Name: "predictivetemp",
	Doc:  "reports a file created at a fixed path in a shared directory",
	Run:  run,
}

// sharedDirs are the temporary directories, which every user can write to, and
// the names macOS gives them.
var sharedDirs = []string{
	"/tmp", "/var/tmp", "/usr/tmp", "/private/tmp", "/private/var/tmp",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			stack = append(stack, n)
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, stack, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a call that creates a file at a fixed path. os.Create also
// gets the fix; os.OpenFile does not.
func check(pass *analysis.Pass, file *ast.File, stack []ast.Node, call *ast.CallExpr) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return
	}
	// os.Root has Create and OpenFile methods, which resolve the path against
	// the root and have no CreateTemp to rewrite to.
	full := fn.FullName()
	if full != "os.Create" && full != "os.OpenFile" {
		return
	}
	literal, ok := constantString(pass, call.Args[0])
	if !ok {
		return
	}
	dir, name, ok := fixedPath(literal)
	if !ok {
		return
	}
	if full == "os.OpenFile" && !creates(pass, fn.Pkg(), call, dir) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "file created at a fixed path in a shared directory",
	}
	if full == "os.Create" {
		text, ok := rewrite(pass, file, call, dir, name)
		if ok && keeps(pass, stack, call) {
			diag.Message += "; write " + text
			if room(pass, file, call, text) {
				diag.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Replace the call with " + text,
					TextEdits: []analysis.TextEdit{{
						Pos:     call.Pos(),
						End:     call.End(),
						NewText: []byte(text),
					}},
				}}
			}
		}
	}
	pass.Report(diag)
}

// keeps reports whether the call still works after the rewrite: the file it
// returns has to be kept, and the path has to be named nowhere else in the
// package.
func keeps(pass *analysis.Pass, stack []ast.Node, call *ast.CallExpr) bool {
	return !discarded(stack) && !repeats(pass, call.Args[0])
}

// room reports whether text may go in the call's place: the call has to lie on
// one line, and no comment may sit inside it.
func room(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, text string) bool {
	if !fits(pass, call.Pos(), call.End(), text) {
		return false
	}
	return !commented(file, call.Pos(), call.End())
}

// discarded reports whether the call's file goes nowhere: it is dropped, or it
// is assigned to no name. The file is the call's first result.
func discarded(stack []ast.Node) bool {
	i := len(stack) - 1
	for i > 0 {
		if _, ok := stack[i-1].(*ast.ParenExpr); !ok {
			break
		}
		i--
	}
	if i == 0 {
		return false
	}
	switch parent := stack[i-1].(type) {
	case *ast.ExprStmt, *ast.DeferStmt, *ast.GoStmt:
		return true
	case *ast.AssignStmt:
		return blank(parent.Lhs[0])
	case *ast.ValueSpec:
		return blank(parent.Names[0])
	}
	return false
}

// blank reports whether expr is the blank identifier.
func blank(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "_"
}

// rewrite returns the text of the os.CreateTemp call that replaces call, and
// reports false when the text would not mean what it says: a * in the name, or
// a bare CreateTemp that another declaration has taken.
func rewrite(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, dir, name string) (string, bool) {
	// os.CreateTemp puts the random string where the last * is, so a name
	// holding one would not survive the rewrite.
	if strings.Contains(name, "*") {
		return "", false
	}
	callee, ok := calleeName(pass, file, call)
	if !ok {
		return "", false
	}
	return callee + "(" + strconv.Quote(dir) +
		", " + strconv.Quote(name) + ")", true
}

// calleeName returns the text that names os.CreateTemp as the call names the
// function it replaces. A selector keeps its qualifier, and a dot import
// writes the bare name.
func calleeName(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (string, bool) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		// The bare name must still resolve to package os at the
		// call.
		if shadowed(pass, file, call.Pos(), "CreateTemp") {
			return "", false
		}
		return "CreateTemp", true
	}
	// The call resolved to a package function, so the qualifier is the
	// package name.
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name + ".CreateTemp", true
}

// shadowed reports whether name resolves to anything other than the function
// of that name in package os at pos.
func shadowed(pass *analysis.Pass, file *ast.File, pos token.Pos, name string) bool {
	scope := pass.TypesInfo.Scopes[file]
	if scope == nil {
		return true
	}
	if scope = scope.Innermost(pos); scope == nil {
		return true
	}
	_, obj := scope.LookupParent(name, pos)
	fn, ok := obj.(*types.Func)
	return !ok || fn.Pkg() == nil || fn.Pkg().Path() != "os"
}

// repeats reports whether an expression other than arg names the same file.
// The rewrite would leave it naming a file the call no longer creates.
func repeats(pass *analysis.Pass, arg ast.Expr) bool {
	value, ok := constantString(pass, arg)
	if !ok {
		return false
	}
	want, ok := key(value)
	if !ok {
		return false
	}
	var found bool
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if found {
				return false
			}
			e, ok := n.(ast.Expr)
			if !ok || e == arg {
				return true
			}
			value, ok := constantString(pass, e)
			if !ok {
				return true
			}
			if got, ok := key(value); ok && got == want {
				found = true
				return false
			}
			return true
		})
	}
	return found
}

// key returns a form of p that compares equal for the two names macOS gives
// one directory, /tmp and /private/tmp. It reports false for a path outside
// the shared directories.
func key(p string) (string, bool) {
	clean := path.Clean(p)
	for _, d := range sharedDirs {
		rest, ok := strings.CutPrefix(clean, d+"/")
		if !ok {
			continue
		}
		return strings.TrimPrefix(d, "/private") + "/" + rest, true
	}
	return "", false
}

// creates reports whether the call can end up with a file another process made
// at the name. The bits come from the os package the code was type-checked
// against, because their values depend on the target platform.
func creates(pass *analysis.Pass, pkg *types.Package, call *ast.CallExpr, dir string) bool {
	flag, ok := intValue(pass, call.Args[1])
	if !ok {
		return false
	}
	create, ok := bits(pkg, "O_CREATE")
	if !ok || flag&create == 0 {
		return false
	}
	// O_EXCL fails instead of opening a file already there, and it protects
	// the last component alone, so it does not settle a path under a
	// subdirectory another process may have made into a link.
	excl, ok := bits(pkg, "O_EXCL")
	return ok && (flag&excl == 0 || !slices.Contains(sharedDirs, dir))
}

// bits returns the value of the named constant in pkg.
func bits(pkg *types.Package, name string) (int64, bool) {
	c, ok := pkg.Scope().Lookup(name).(*types.Const)
	if !ok {
		return 0, false
	}
	return constant.Int64Val(c.Val())
}

// intValue returns the value of e when it is an integer constant.
func intValue(pass *analysis.Pass, e ast.Expr) (int64, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

// constantString returns the value of e when it is a string constant, which
// covers a literal, a named constant, and a concatenation of constants.
func constantString(pass *analysis.Pass, e ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// fixedPath splits a literal path into the directory it names and the file it
// creates, and reports false unless that directory is shared. A path holding a
// ".." is refused: cleaning it is lexical, and a link in the way would make
// the result a different file than the one the call opens.
func fixedPath(p string) (dir, name string, ok bool) {
	if slices.Contains(strings.Split(p, "/"), "..") {
		return "", "", false
	}
	p = path.Clean(p)
	if !path.IsAbs(p) {
		return "", "", false
	}
	dir, name = path.Dir(p), path.Base(p)
	return dir, name, shared(dir)
}

// shared reports whether dir is a shared directory or lies under one.
func shared(dir string) bool {
	for _, d := range sharedDirs {
		if dir == d || strings.HasPrefix(dir, d+"/") {
			return true
		}
	}
	return false
}

// commented reports whether a comment sits in the span a fix would replace,
// where it would be dropped with the call.
func commented(file *ast.File, from, to token.Pos) bool {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if c.Pos() >= from && c.End() <= to {
				return true
			}
		}
	}
	return false
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
