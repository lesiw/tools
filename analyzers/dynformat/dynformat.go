// Package dynformat provides an analysis.Analyzer that reports a
// printf-style call with a non-constant format string and no argument after
// it.
//
// The fmt doc describes Printf as formatting according to a format specifier
// and Print as formatting its operands with their default formats. The
// suite's printf pass reports a non-constant format string with no other
// arguments in a call to a printf-style function in a file that declares
// Go 1.24 or later. The pass doc says such a call may have been intended
// for the function's Print-like counterpart. With no other arguments, the
// plain function or method leaves the string unchanged. The fix renames the
// function or method in the call to that counterpart. This analyzer has no
// language-version gate: the rule applies to every file it checks.
//
// The testing Logf, Errorf, Fatalf, and Skipf methods are left out: their
// plain forms record through Sprintln, which adds a newline before the final
// newline is trimmed, so a value that ends in a newline gains a blank line.
//
// runtime/trace.Logf is left out: Log reads the context before checking
// whether tracing is enabled, while Logf returns first. The rename changes
// behavior with tracing off.
//
// fmt.Errorf is left alone. The only rewrite that drops the format-string
// parameter is errors.New. errfmt already reports errors.New as replaceable
// by fmt.Errorf and gives no fix, so renaming here would replace one
// diagnostic with another that has no fix.
//
// In a file that declares Go 1.24 or later, the printf pass reports the
// testing and trace calls and fmt.Errorf; its "%s" fix writes the value as
// text.
//
// The rule is adapted from staticcheck's SA1006, which reports a dynamic
// format string with no further arguments and rewrites the call. The README
// lists the calls this analyzer covers.
package dynformat

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "dynformat",
	Doc:  "reports a printf-style call with a non-constant format string and no argument after it",
	Run:  run,
}

type replacement struct {
	plain       string // the name of the plain function or method
	formatIndex int    // the index of the format string among the arguments
}

// replacements maps a formatted function or method to the plain function or
// method that takes its place. A method's package is the one that declares it,
// so the log methods match their package's keys.
var replacements = map[string]replacement{
	"fmt.Appendf": {plain: "Append", formatIndex: 1},
	"fmt.Fprintf": {plain: "Fprint", formatIndex: 1},
	"fmt.Printf":  {plain: "Print", formatIndex: 0},
	"fmt.Sprintf": {plain: "Sprint", formatIndex: 0},
	"log.Fatalf":  {plain: "Fatal", formatIndex: 0},
	"log.Panicf":  {plain: "Panic", formatIndex: 0},
	"log.Printf":  {plain: "Print", formatIndex: 0},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, call *ast.CallExpr) {
	fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
	if !ok || fn.Pkg() == nil {
		return
	}
	repl, ok := replacements[fn.Pkg().Path()+"."+fn.Name()]
	if !ok {
		return
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	// A method expression passes the receiver first, so the format index is
	// one higher.
	formatIndex := repl.formatIndex
	selection := pass.TypesInfo.Selections[sel]
	if selection != nil && selection.Kind() == types.MethodExpr {
		formatIndex++
	}
	var dynamicFormat bool
	if len(call.Args) == 1 {
		tuple, ok := pass.TypesInfo.TypeOf(call.Args[0]).(*types.Tuple)
		if ok {
			if tuple.Len() != formatIndex+1 {
				return
			}
			dynamicFormat = true
		}
	}
	if !dynamicFormat {
		if len(call.Args) != formatIndex+1 {
			return
		}
		if !dynamic(pass, call.Args[formatIndex]) {
			return
		}
	}
	if !resolves(pass, sel.X, repl.plain, fn) {
		return
	}
	d := analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "format string is not constant; use " + repl.plain,
	}
	if text, ok := rename(pass, call, sel, repl.plain); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace with " + repl.plain,
			TextEdits: []analysis.TextEdit{{
				Pos:     call.Pos(),
				End:     call.End(),
				NewText: text,
			}},
		}}
	}
	pass.Report(d)
}

// rename returns the call's source with the selector renamed. The edit covers
// the whole call. The printf pass inserts "%s", and the space after it before
// the format operand of the same call, and two edits that did not overlap
// would write fmt.Print("%s", s). The whole-call edit overlaps the
// insertion, so the insertion is skipped for that round.
func rename(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr, name string) ([]byte, bool) {
	file := pass.Fset.File(call.Pos())
	if file == nil {
		return nil, false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return nil, false
	}
	lo, hi := file.Offset(call.Pos()), file.Offset(call.End())
	from, to := file.Offset(sel.Sel.Pos()), file.Offset(sel.Sel.End())
	if lo < 0 || from < lo || to > hi || hi > len(src) {
		return nil, false
	}
	return slices.Concat(src[lo:from], []byte(name), src[to:hi]), true
}

// dynamic reports whether arg is a format string whose value is not known at
// the call.
func dynamic(pass *analysis.Pass, arg ast.Expr) bool {
	tv, ok := pass.TypesInfo.Types[arg]
	return ok && tv.Value == nil
}

// resolves reports whether name, written after e, names a function or method
// in the package that declares fn. A type that declares a Print method, for
// example, shadows the method log declares, so the rename would call that
// method.
func resolves(pass *analysis.Pass, e ast.Expr, name string, fn *types.Func) bool {
	var obj types.Object
	if id, ok := e.(*ast.Ident); ok {
		if pkg, isPkg := pass.TypesInfo.Uses[id].(*types.PkgName); isPkg {
			obj = pkg.Imported().Scope().Lookup(name)
		}
	}
	if obj == nil {
		recv := pass.TypesInfo.TypeOf(e)
		if recv == nil {
			return false
		}
		obj, _, _ = types.LookupFieldOrMethod(recv, true, nil, name)
	}
	target, ok := obj.(*types.Func)
	return ok && target.Pkg() == fn.Pkg()
}
