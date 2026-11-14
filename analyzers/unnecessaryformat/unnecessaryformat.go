// Package unnecessaryformat provides an analysis.Analyzer that reports a call
// whose format string has no verb.
//
// t.Errorf("the list is empty") is a format call with nothing to format.
//
// The fmt doc separates the two kinds of call: Printf "formats according to a
// format specifier", while Print and Println format their operands with the
// default formats, Println appending a newline. Sprint and Sprintln return
// the text instead of writing it, and Fprint and Fprintln write it to a
// writer. testing's doc says Errorf "is equivalent to Logf followed by Fail",
// that Logf's "final newline is added if not provided", and that Log formats
// its arguments the way Println does.
//
// The fix renames the call to the function that takes no format string, and
// takes a trailing newline off a literal that spells it \n, which the ln name
// puts back: fmt.Printf("done\n") becomes fmt.Println("done"). Error has no
// ln form, so its text loses the newline and records the same text:
// t.Errorf("x\n\n") becomes t.Error("x\n").
//
// The rule is like revive's unnecessary-format, which reads a literal format
// with no verb and leaves the edit to the reader. This analyzer makes the
// edit. The README lists the calls and the cases the fix leaves alone.
package unnecessaryformat

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unnecessaryformat",
	Doc:  "reports a call whose format string has no verb",
	Run:  run,
}

type replacement struct {
	plain       string // the name of the function that takes no format string
	ln          string // the name of the plain function's newline form
	formatIndex int    // the index of the format string among the arguments
}

// replacements maps a formatted function to the plain function that takes its
// place.
var replacements = map[string]replacement{
	"fmt.Printf":  {plain: "Print", ln: "Println", formatIndex: 0},
	"fmt.Sprintf": {plain: "Sprint", ln: "Sprintln", formatIndex: 0},
	"fmt.Fprintf": {plain: "Fprint", ln: "Fprintln", formatIndex: 1},
	// Error records through Sprintln, so it has no newline form: the trailing
	// newline comes off the text instead.
	"testing.Errorf": {plain: "Error", formatIndex: 0},
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
	fn, sel := called(pass, call)
	if fn == nil {
		return
	}
	repl, ok := replacements[fn.Pkg().Path()+"."+fn.Name()]
	if !ok {
		return
	}
	// A call that passes arguments after the format string is left alone: fmt
	// prints them as a marker, and the rename would print them as operands.
	if len(call.Args) != repl.formatIndex+1 {
		return
	}
	arg := call.Args[repl.formatIndex]
	text, ok := constantText(pass, arg)
	if !ok {
		return
	}
	// A percent sign leaves the call alone: Printf reads one as the start of a
	// directive and Print copies it, so the rename would write other bytes.
	if strings.Contains(text, "%") {
		return
	}
	sibling, edits, ok := fix(arg, sel.Sel, text, repl)
	if !ok || !resolves(pass, sel.X, sibling, fn) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "format string has no verb; use " + sibling,
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "Replace with " + sibling,
			TextEdits: edits,
		}},
	})
}

// called returns the function a call invokes and the selector that spells it,
// or nil when the callee has no package, as for the method error.Error.
func called(pass *analysis.Pass, call *ast.CallExpr) (*types.Func, *ast.SelectorExpr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	fn, _ := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if fn == nil || fn.Pkg() == nil {
		return nil, nil
	}
	return fn, sel
}

// resolves reports whether name, written after e, names a function in the
// package that declares fn. A type that declares its own Error shadows it, so
// the rename would call that method.
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

// constantText returns the text of a constant string argument, and reports
// whether the argument is one.
func constantText(pass *analysis.Pass, arg ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// fix returns the name to report and the edits that write it, and reports
// whether a fix exists.
func fix(arg ast.Expr, name *ast.Ident, text string, repl replacement) (string, []analysis.TextEdit, bool) {
	source, before, shed := shedNewline(arg, text)
	if shed && repl.ln != "" && !readable(before) {
		shed = false
	}
	// A name that records through Sprintln has no newline form, so a text that
	// ends in a newline the edit cannot shed would record a blank line the
	// formatted call does not.
	if !shed && repl.ln == "" && strings.HasSuffix(text, "\n") {
		return "", nil, false
	}
	var edits []analysis.TextEdit
	sibling := repl.plain
	if shed && repl.ln != "" {
		sibling = repl.ln
	}
	edits = append(edits, analysis.TextEdit{
		Pos:     name.Pos(),
		End:     name.End(),
		NewText: []byte(sibling),
	})
	if shed {
		edits = append(edits, analysis.TextEdit{
			Pos:     arg.Pos(),
			End:     arg.End(),
			NewText: []byte(source),
		})
	}
	return sibling, edits, true
}

// shedNewline returns the literal's source with the newline that ends text
// removed, the text before that newline, and whether the newline came off. The
// literal value must end in \n and the closing quote, so the last three bytes
// can come off; a newline spelled \x0a or \012 is left alone.
func shedNewline(arg ast.Expr, text string) (source, before string, shed bool) {
	lit, isLit := arg.(*ast.BasicLit)
	if !isLit {
		return "", "", false
	}
	before, ends := strings.CutSuffix(text, "\n")
	if !ends || before == "" {
		return "", "", false
	}
	if !strings.HasSuffix(lit.Value, `\n"`) {
		return "", "", false
	}
	return lit.Value[:len(lit.Value)-3] + `"`, before, true
}

// readable reports whether the text in front of a newline can stay in front of
// an ln name. A space, a tab, a carriage return, or another newline there
// would hide where the text ends and the name's newline begins, though both
// write the same bytes.
func readable(before string) bool {
	switch before[len(before)-1] {
	case ' ', '\t', '\r', '\n':
		return false
	}
	return true
}
