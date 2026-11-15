// Package nowsub provides an analysis.Analyzer that reports time.Now().Sub(t)
// or t.Sub(time.Now()).
//
// The time package states the shorthand for each direction: "Since returns the
// time elapsed since t. It is shorthand for time.Now().Sub(t)" and "Until
// returns the duration until t. It is shorthand for t.Sub(time.Now())".
//
// In the Since direction a call whose argument can have an effect is left
// alone, because Since reads the clock after its argument. A conversion is
// free.
//
// The fix writes Since or Until with the name the file gives the time package,
// and is withheld where the edit would join lines or drop a comment. A
// replacement that prints on more than one line is reported without its
// argument, and takes no fix.
//
// The rule is adapted from staticcheck's S1012 and S1024, which do not check
// the Since argument. The README catalogs every check, exemption, and fix.
package nowsub

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "nowsub",
	Doc:  "reports time.Now().Sub(t) or t.Sub(time.Now())",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, file, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a Sub call that Since or Until writes more directly.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 || !isTimeFunc(pass, sel.Sel, "Sub") {
		return
	}
	if qual, ok := nowQualifier(pass, sel.X); ok {
		// Since reads the clock after its argument, so the rewrite would
		// move that read past whatever the argument does.
		if astexpr.Effectful(pass, call.Args[0]) {
			return
		}
		if text, ok := arg(pass, call.Args[0]); ok {
			report(pass, file, call, qual+".Since", text)
		}
		return
	}
	if qual, ok := nowQualifier(pass, call.Args[0]); ok {
		if text, ok := arg(pass, sel.X); ok {
			report(pass, file, call, qual+".Until", text)
		}
	}
}

// report emits the diagnostic for call, replaced by name(argText). A
// diagnostic prints on one line, so a rewrite that prints on more than one is
// reported as a call to the function, without its argument. It takes no fix,
// because the edit would leave the printer's line breaks for fmt.
func report(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, name, argText string) {
	var (
		text = name + "(" + argText + ")"
		diag = analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "call can be replaced by " + text,
		}
	)
	if strings.ContainsRune(text, '\n') {
		diag.Message = "call can be replaced by a call to " + name
		pass.Report(diag)
		return
	}
	if fixable(pass.Fset, file, call) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Replace with " + text,
			TextEdits: []analysis.TextEdit{{
				Pos:     call.Pos(),
				End:     call.End(),
				NewText: []byte(text),
			}},
		}}
	}
	pass.Report(diag)
}

// fixable reports whether call lies on one line and holds no comment, so that
// the rewrite neither joins lines nor drops a comment. It reads written
// positions, which a //line directive does not move. Since and Until are
// shorter than the calls they replace, so the fix never takes the line past
// the width.
func fixable(fset *token.FileSet, file *ast.File, call *ast.CallExpr) bool {
	var (
		start = fset.PositionFor(call.Pos(), false)
		end   = fset.PositionFor(call.End(), false)
	)
	if start.Line != end.Line {
		return false
	}
	for _, group := range file.Comments {
		if group.Pos() < call.End() && call.Pos() < group.End() {
			return false
		}
	}
	return true
}

// nowQualifier returns the name the file gives the time package when e is a
// call to time.Now.
func nowQualifier(pass *analysis.Pass, e ast.Expr) (string, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return "", false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || !isTimeFunc(pass, sel.Sel, "Now") {
		return "", false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, true
}

// arg returns the text to write in place of e, and false when e is not a
// time.Time or a *time.Time. A *time.Time is written as the value it points
// at.
func arg(pass *analysis.Pass, e ast.Expr) (string, bool) {
	var (
		expr = ast.Unparen(e)
		text = astexpr.Text(expr)
		t    = pass.TypesInfo.TypeOf(e)
	)
	if !isTimeValue(t) {
		if !isTimePointer(t) {
			return "", false
		}
		unary, ok := expr.(*ast.UnaryExpr)
		if ok && unary.Op == token.AND {
			text = astexpr.Text(ast.Unparen(unary.X))
		} else {
			text = "*" + text
		}
	}
	return text, true
}

// isTimeFunc reports whether id is the function called name in package time.
func isTimeFunc(pass *analysis.Pass, id *ast.Ident, name string) bool {
	fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
	return ok && fn.Name() == name && isTimePkg(fn.Pkg())
}

// isTimeValue reports whether t is time.Time.
func isTimeValue(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj().Name() == "Time" && isTimePkg(named.Obj().Pkg())
}

// isTimePointer reports whether t is a *time.Time.
func isTimePointer(t types.Type) bool {
	ptr, ok := types.Unalias(t).(*types.Pointer)
	return ok && isTimeValue(ptr.Elem())
}

// isTimePkg reports whether pkg is the time package.
func isTimePkg(pkg *types.Package) bool {
	return pkg != nil && pkg.Path() == "time"
}
