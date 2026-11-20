// Package sprintfprint provides an analysis.Analyzer that reports a
// fmt.Sprintf call whose result a print, log, or testing call formats again.
//
// The fmt package doc says Print "formats using the default formats for its
// operands" and Printf "formats according to a format specifier". A string
// formats as itself, so fmt.Print writes the text of one fmt.Sprintf call, and
// fmt.Printf writes that text from the format and the operands.
//
// The calls are fmt's Fprint, Fprintln, Print, Println, and Sprintln; log's
// Print, Println, Fatal, Fatalln, Panic, and Panicln, package-level and on a
// *log.Logger or a type that embeds one; and testing's Error, Fatal, Log, and
// Skip, on a *testing.T, a *testing.B, a *testing.F, a testing.TB, or a type
// that embeds one. A method counts as a call of the package that declares it,
// and the f form must resolve to that package at the call site. fmt.Sprint is
// left to redundantsprint, which drops the same wrapper.
//
// The fix writes the newline a call ends its message with into the format,
// renames the call, and drops the fmt.Sprintf around the message.
//
// The README lists the cases the fix leaves alone.
//
// The rule is adapted from staticcheck's S1038, which reports these wrappers
// without a fix, and leaves an ln call whose format is not a literal alone.
package sprintfprint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "sprintfprint",
	Doc:  "reports a Sprintf result passed to a print call",
	Run:  run,
}

// wrapper describes a call that takes an already-formatted message and has an
// f form.
type wrapper struct {
	arity       int  // the number of arguments the call takes
	addsNewline bool // the call ends the message with a newline
}

// wrappers holds the calls that an f form takes over, keyed by the package
// that declares them and the name it declares there. The message is the last
// argument.
var wrappers = map[string]wrapper{
	"fmt.Fprint":    {arity: 2},
	"fmt.Fprintln":  {arity: 2, addsNewline: true},
	"fmt.Print":     {arity: 1},
	"fmt.Println":   {arity: 1, addsNewline: true},
	"fmt.Sprintln":  {arity: 1, addsNewline: true},
	"log.Fatal":     {arity: 1},
	"log.Fatalln":   {arity: 1, addsNewline: true},
	"log.Panic":     {arity: 1},
	"log.Panicln":   {arity: 1, addsNewline: true},
	"log.Print":     {arity: 1},
	"log.Println":   {arity: 1, addsNewline: true},
	"testing.Error": {arity: 1, addsNewline: true},
	"testing.Fatal": {arity: 1, addsNewline: true},
	"testing.Log":   {arity: 1, addsNewline: true},
	"testing.Skip":  {arity: 1, addsNewline: true},
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

// check reports call when its message argument is a fmt.Sprintf call and the
// package that declares call has the f form of it.
func check(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) {
	ident, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return
	}
	w, ok := wrappers[fn.Pkg().Path()+"."+fn.Name()]
	if !ok {
		return
	}
	arity := w.arity
	if methodExpression(pass, call.Fun) {
		arity++ // the receiver comes first
	}
	if len(call.Args) != arity {
		return
	}
	msg := call.Args[arity-1]
	sprintf, ok := ast.Unparen(msg).(*ast.CallExpr)
	if !ok || !formats(pass, sprintf) {
		return
	}
	name := strings.TrimSuffix(fn.Name(), "ln") + "f"
	if !resolves(pass, call, name, fn) {
		return
	}
	// A multi-value call fills an argument list of its own, so the message can
	// stand in only where it becomes the new call's whole list: no writer
	// before it, and no newline appended to it.
	_, multi := pass.TypesInfo.Types[sprintf.Args[0]].Type.(*types.Tuple)
	standIn := !multi || (arity == 1 && !w.addsNewline)
	called := named(call.Fun, fn.Name())
	shown := named(call.Fun, name)
	text := "Sprintf result passed to " + called + "; use " + shown
	if w.addsNewline {
		text += " with the newline in the format"
	}
	var fix []analysis.SuggestedFix
	if standIn {
		fix = fixes(file, ident, msg, sprintf, name, w.addsNewline)
	}
	pass.Report(analysis.Diagnostic{
		Pos:            call.Pos(),
		End:            call.End(),
		Message:        text,
		SuggestedFixes: fix,
	})
}

// methodExpression reports whether fun names a method as a value, which takes
// the receiver as its first argument.
func methodExpression(pass *analysis.Pass, fun ast.Expr) bool {
	sel, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	selection := pass.TypesInfo.Selections[sel]
	return selection != nil && selection.Kind() == types.MethodExpr
}

// formats reports whether call is a call to fmt.Sprintf.
func formats(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return false
	}
	return fn.Pkg().Path() == "fmt" && fn.Name() == "Sprintf"
}

// resolves reports whether name reaches a function of the package that
// declares fn at the call site. A type or a variable that declares the name
// itself shadows the package's, and the rewrite would call that instead.
func resolves(pass *analysis.Pass, call *ast.CallExpr, name string, fn *types.Func) bool {
	sel, qualified := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !qualified {
		// A dot import puts the package's names in the file scope, where a
		// declaration at the call site can shadow one.
		scope := pass.Pkg.Scope().Innermost(call.Pos())
		if scope == nil {
			return false
		}
		_, obj := scope.LookupParent(name, call.Pos())
		target, ok := obj.(*types.Func)
		return ok && target.Pkg() == fn.Pkg()
	}
	if ident, ok := sel.X.(*ast.Ident); ok {
		if pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName); ok {
			_, declared := pkg.Imported().Scope().Lookup(name).(*types.Func)
			return declared
		}
	}
	tv, ok := pass.TypesInfo.Types[sel.X]
	if !ok {
		return false
	}
	recv := tv.Type
	obj, _, _ := types.LookupFieldOrMethod(
		recv, tv.Addressable(),
		pass.Pkg, name,
	)
	target, ok := obj.(*types.Func)
	return ok && target.Pkg() == fn.Pkg()
}

// fixes returns the edit that renames the call and drops the Sprintf, and
// nothing when a comment sits in the text the edit deletes.
func fixes(file *ast.File, ident *ast.Ident, msg ast.Expr, sprintf *ast.CallExpr, name string, addsNewline bool) []analysis.SuggestedFix {
	var (
		last = sprintf.Args[len(sprintf.Args)-1].End()
		open = analysis.TextEdit{
			Pos: msg.Pos(),
			End: sprintf.Args[0].Pos(),
		}
		shut = analysis.TextEdit{Pos: last, End: msg.End()}
	)
	// A spread belongs to the call that takes over, so the edit starts after
	// the three dots.
	if sprintf.Ellipsis.IsValid() {
		shut.Pos = sprintf.Ellipsis + 3
	}
	for _, span := range []analysis.TextEdit{open, shut} {
		if commented(file.Comments, span.Pos, span.End) {
			return nil
		}
	}
	edits := []analysis.TextEdit{
		{Pos: ident.Pos(), End: ident.End(), NewText: []byte(name)},
		open,
		shut,
	}
	if addsNewline {
		edits = append(edits, newline(sprintf.Args[0]))
	}
	return []analysis.SuggestedFix{{
		Message:   "Replace with " + name,
		TextEdits: edits,
	}}
}

// newline returns the edit that ends the format's text with a newline. An
// interpreted string literal takes the escape before its closing quote; any
// other format is concatenated, which the f form formats in the same place.
func newline(format ast.Expr) analysis.TextEdit {
	lit, ok := format.(*ast.BasicLit)
	if ok && lit.Kind == token.STRING && strings.HasPrefix(lit.Value, `"`) {
		return analysis.TextEdit{
			Pos:     lit.End() - 1,
			End:     lit.End() - 1,
			NewText: []byte(`\n`),
		}
	}
	return analysis.TextEdit{
		Pos:     format.End(),
		End:     format.End(),
		NewText: []byte(` + "\n"`),
	}
}

// commented reports whether a comment overlaps the text from pos to end.
func commented(comments []*ast.CommentGroup, pos, end token.Pos) bool {
	for _, group := range comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}

// named returns the qualifier or receiver the syntax tree prints and name,
// joined as a selector: fmt.Printf, t.Errorf.
func named(fun ast.Expr, name string) string {
	sel, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok {
		return name
	}
	return types.ExprString(sel.X) + "." + name
}
