// Package templateparse provides an analysis.Analyzer that reports a template
// that does not parse.
//
// The text/template package documents Must as a helper that "wraps a call to a
// function returning (*Template, error) and panics if the error is non-nil"
// (https://pkg.go.dev/text/template#Must), for initializations such as
// template.Must(template.New("name").Parse("text")). A constant the parser
// rejects raises that error wherever the call runs, and template.Must turns it
// into a panic.
//
// Only a constant text is checked. The rule follows a template from the New
// call that builds it, through the methods that return a template, so the
// delimiters Delims sets along the way are the ones the text is parsed with. A
// template held in a variable or a field is not checked, because its
// delimiters may have been set where the call cannot be seen.
//
// A call to a function the template does not define is reported, unless the
// template was built with Funcs, whose map the analyzer does not read.
//
// The diagnostic carries no fix. Making a template parse needs the text its
// author meant, which the analyzer cannot supply.
//
// Adapted from staticcheck's SA1001, which reports the same fault at a call to
// Parse on the result of New. This rule reads the delimiters that Delims sets,
// and reports the parse errors upstream leaves alone.
package templateparse

import (
	"go/ast"
	"go/constant"
	"go/types"
	htmltemplate "html/template"
	"strconv"
	"strings"
	texttemplate "text/template"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "templateparse",
	Doc:  "reports a template that does not parse",
	Run:  run,
}

// template is the state a Parse call sees: the action delimiters in force and
// whether a function map was registered along the way.
type template struct {
	left, right string
	funcs       bool
}

// templateDefaults are the delimiters a new template starts with.
var templateDefaults = template{left: "{{", right: "}}"}

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

// check reports a call to Parse that a constant makes fail.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	if len(call.Args) != 1 {
		return
	}
	pkg, expr, ok := parseCall(pass, call)
	if !ok {
		return
	}
	text, ok := constantString(pass, call.Args[0])
	if !ok {
		return
	}
	tpl, ok := chain(pass, expr, pkg)
	if !ok {
		return
	}
	err := parse(pkg, text, tpl)
	if err == nil {
		return
	}
	line, msg, ok := detail(err)
	if !ok {
		return
	}
	if undefinedFunction(msg) && tpl.funcs {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Args[0].Pos(),
		End:     call.Args[0].End(),
		Message: message(line, msg),
	})
}

// parseCall returns the package of a call to Parse on a template, and the
// template it parses into.
func parseCall(pass *analysis.Pass, call *ast.CallExpr) (string, ast.Expr, bool) {
	fn, ok := templateFunc(pass, call)
	if !ok || fn.Name() != "Parse" || !templateMethod(fn) {
		return "", nil, false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", nil, false
	}
	return fn.Pkg().Path(), sel.X, true
}

// templateFunc returns the function a call names, when text/template or
// html/template declares it.
func templateFunc(pass *analysis.Pass, call *ast.CallExpr) (*types.Func, bool) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok || fn.Pkg() == nil {
		return nil, false
	}
	switch fn.Pkg().Path() {
	case "text/template", "html/template":
		return fn, true
	}
	return nil, false
}

// templateMethod reports whether fn is a method on a template.
func templateMethod(fn *types.Func) bool {
	if fn.Signature().Recv() == nil {
		return false
	}
	recv := fn.Signature().Recv().Type()
	if pointer, ok := recv.(*types.Pointer); ok {
		recv = pointer.Elem()
	}
	named, ok := recv.(*types.Named)
	return ok && named.Obj().Name() == "Template"
}

// chain returns the state of a template expression. A template that does not
// trace back to a call to New has delimiters this function cannot name, and it
// reports false for one.
func chain(pass *analysis.Pass, expr ast.Expr, pkg string) (template, bool) {
	var (
		tpl   template
		known bool
	)
	for {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		if !ok {
			return template{}, false
		}
		fn, ok := templateFunc(pass, call)
		if !ok || fn.Pkg().Path() != pkg {
			return template{}, false
		}
		if fn.Signature().Recv() == nil {
			// Only New starts a template. Every other function in the
			// package takes one it did not build.
			if fn.Name() != "New" {
				return template{}, false
			}
			if !known {
				tpl.left, tpl.right = templateDefaults.left,
					templateDefaults.right
			}
			return tpl, true
		}
		if !templateMethod(fn) {
			return template{}, false
		}
		switch fn.Name() {
		case "Delims":
			// The Delims nearest the call is the one in force.
			if !known {
				var ok bool
				tpl, ok = setDelims(pass, call, tpl)
				if !ok {
					return template{}, false
				}
				known = true
			}
		case "Funcs":
			tpl.funcs = true
		}
		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !ok {
			return template{}, false
		}
		expr = sel.X
	}
}

// setDelims returns t with the delimiters of a call to Delims, which takes two
// constants.
func setDelims(pass *analysis.Pass, call *ast.CallExpr, t template) (template, bool) {
	if len(call.Args) != 2 {
		return template{}, false
	}
	left, ok := constantString(pass, call.Args[0])
	if !ok {
		return template{}, false
	}
	right, ok := constantString(pass, call.Args[1])
	if !ok {
		return template{}, false
	}
	t.left, t.right = left, right
	return t, true
}

// constantString returns the value of e when e is a constant string.
func constantString(pass *analysis.Pass, e ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// parse parses text the way the program will, and returns the error the
// package reports for it.
func parse(pkg, text string, t template) error {
	if pkg == "html/template" {
		_, err := htmltemplate.New("").Delims(t.left, t.right).Parse(text)
		return err
	}
	_, err := texttemplate.New("").Delims(t.left, t.right).Parse(text)
	return err
}

// undefinedFunction reports whether msg is the parse error for a function the
// template does not define.
func undefinedFunction(msg string) bool {
	return strings.HasPrefix(msg, "function ") &&
		strings.HasSuffix(msg, " not defined")
}

// detail returns the line of the text a parse error was found on and the
// error's message, without the name and line the parse package adds.
func detail(err error) (int, string, bool) {
	rest, ok := strings.CutPrefix(err.Error(), "template: :")
	if !ok {
		return 0, "", false
	}
	line, msg, ok := strings.Cut(rest, ": ")
	if !ok {
		return 0, "", false
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		return 0, "", false
	}
	// A template defined twice is reported with a prefix of its own.
	return n, strings.TrimPrefix(msg, "template: "), true
}

// message is the diagnostic for a parse error found on line n of a template.
func message(n int, msg string) string {
	if n == 1 {
		return "template does not parse: " + msg
	}
	return "template does not parse at line " + strconv.Itoa(n) + ": " + msg
}
