// Package nowsub provides an analysis.Analyzer that reports time.Now().Sub(t)
// and t.Sub(time.Now()).
//
// The time package states the shorthand for each direction: "Since returns the
// time elapsed since t. It is shorthand for time.Now().Sub(t)" and "Until
// returns the duration until t. It is shorthand for t.Sub(time.Now())".
//
// In time.Now().Sub(x) the receiver must be time.Now() itself, so
// time.Now().Add(d).Sub(t) is left alone. In x.Sub(time.Now()) the receiver
// must be a time.Time or a *time.Time; a struct that embeds time.Time promotes
// Sub, and that call is left alone. A call is examined only when Now is
// package-qualified, so a file that dot-imports time is not reported.
//
// The rule is adapted from staticcheck's S1012 and S1024, which report the two
// directions separately.
//
// The fix writes Since or Until with the name the file gives the time
// package, so a renamed import keeps its name. A receiver of type *time.Time
// is written as the value it points at, so p becomes *p and &start becomes
// start.
// It is withheld when the replaced call spans lines or holds a comment. In the
// Since direction a call is reported only when the argument does no work,
// because the rewrite reads the clock after the argument.
package nowsub

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "nowsub",
	Doc:  "reports time.Now().Sub(t) or t.Sub(time.Now())",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, f, call)
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
	if qual, ok := now(pass, sel.X); ok {
		// Since reads the clock after its argument, so the rewrite would
		// move that read past whatever the argument does.
		if astexpr.Effectful(pass, call.Args[0]) {
			return
		}
		if text, ok := arg(pass, call.Args[0]); ok {
			report(pass, file, call, qual+"Since("+text+")")
		}
	} else if qual, ok := now(pass, call.Args[0]); ok {
		if text, ok := arg(pass, sel.X); ok {
			report(pass, file, call, qual+"Until("+text+")")
		}
	}
}

// report emits the diagnostic for call, which text replaces.
func report(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, text string) {
	pass.Report(analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        "call can be replaced by " + text,
		SuggestedFixes: fix(pass, file, call, text),
	})
}

// fix returns the edit that writes text, or none when the call cannot take an
// edit.
func fix(pass *analysis.Pass, file *ast.File, call *ast.CallExpr, text string) []analysis.SuggestedFix {
	if !fixable(pass.Fset, file, call) {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message: "Replace with " + text,
		TextEdits: []analysis.TextEdit{{
			Pos:     call.Pos(),
			End:     call.End(),
			NewText: []byte(text),
		}},
	}}
}

// now reports whether e is a call to time.Now, and returns the text that names
// the time package as the file writes it.
func now(pass *analysis.Pass, e ast.Expr) (string, bool) {
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
	if _, ok := pass.TypesInfo.Uses[id].(*types.PkgName); !ok {
		return "", false
	}
	return id.Name + ".", true
}

// arg returns the text of e as the argument of Since or Until, which take a
// time.Time. A pointer to one is written as the value it points at, so p
// becomes *p and &start becomes start.
func arg(pass *analysis.Pass, e ast.Expr) (string, bool) {
	t := types.Unalias(pass.TypesInfo.TypeOf(e))
	if isTimeValue(t) {
		return astexpr.Text(ast.Unparen(e)), true
	}
	ptr, ok := t.(*types.Pointer)
	if !ok || !isTimeValue(types.Unalias(ptr.Elem())) {
		return "", false
	}
	unary, ok := ast.Unparen(e).(*ast.UnaryExpr)
	if ok && unary.Op == token.AND {
		return astexpr.Text(unary.X), true
	}
	return "*" + astexpr.Text(ast.Unparen(e)), true
}

// isTimeFunc reports whether id names the function of package time called
// name.
func isTimeFunc(pass *analysis.Pass, id *ast.Ident, name string) bool {
	fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
	return ok && fn.Name() == name && isTimePkg(fn.Pkg())
}

// isTimeValue reports whether t is time.Time.
func isTimeValue(t types.Type) bool {
	named, ok := t.(*types.Named)
	return ok && named.Obj().Name() == "Time" && isTimePkg(named.Obj().Pkg())
}

// isTimePkg reports whether pkg is the time package.
func isTimePkg(pkg *types.Package) bool {
	return pkg != nil && pkg.Path() == "time"
}

// fixable reports whether call lies on one line and holds no comment, so that
// the rewrite neither joins lines nor drops a comment. Written positions are
// used, which a //line directive does not move. No width check is needed:
// every rewrite is shorter than the call it replaces.
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
