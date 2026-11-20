// Package nilctx provides an analysis.Analyzer that reports a nil argument
// for a context.Context first parameter.
//
// The context package's doc says "Do not pass a nil Context, even if a
// function permits it. Pass context.TODO if you are unsure about which
// Context to use" (https://pkg.go.dev/context).
//
// A call is reported when its first parameter is context.Context and the
// argument written for that parameter is nil. A method expression passes the
// receiver as its first argument, and the receiver is not a parameter, so the
// rule reads the argument after it. A conversion is not a call, and the rule
// does not follow a call through a variable. Only the standard context.Context
// is reported: a parameter of any other type, a defined type whose underlying
// type is context.Context among them, is left alone.
//
// The fix writes context.TODO() under the import name the file uses, adding
// the import where the file has none. It is withheld for a file that
// dot-imports or blank-imports context, for a call where another declaration
// holds the name, and where the rewrite would take the line past the width.
// The call is reported either way.
//
// The rule is adapted from staticcheck's SA1012.
package nilctx

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/imports"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilctx",
	Doc:  "reports a nil argument for a context.Context first parameter",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if arg, ok := contextArg(pass, call); ok {
				report(pass, file, arg)
			}
			return true
		})
	}
	return nil, nil
}

// contextArg returns the argument written for a call's first parameter, when
// that parameter is a context.Context and the argument is nil.
func contextArg(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok {
		return nil, false
	}
	params := fn.Signature().Params()
	if params.Len() == 0 || !isContext(params.At(0).Type()) {
		return nil, false
	}
	arg := call.Args[firstArg(pass, call)]
	if !pass.TypesInfo.Types[ast.Unparen(arg)].IsNil() {
		return nil, false
	}
	return arg, true
}

// firstArg returns the index in call.Args of the argument written for the
// first parameter. An argument list starts with the receiver when the function
// is a method expression, as in (*T).Stop(nil).
func firstArg(pass *analysis.Pass, call *ast.CallExpr) int {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return 0
	}
	selection := pass.TypesInfo.Selections[sel]
	if selection != nil && selection.Kind() == types.MethodExpr {
		return 1
	}
	return 0
}

// isContext reports whether t is context.Context or an alias of it.
func isContext(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "context" &&
		obj.Name() == "Context"
}

// report diagnoses arg, with the fix that writes context.TODO() where the
// file can name it.
func report(pass *analysis.Pass, file *ast.File, arg ast.Expr) {
	var (
		diag = analysis.Diagnostic{
			Pos:     arg.Pos(),
			End:     arg.End(),
			Message: "the context.Context argument is nil",
		}
		edits, text, ok = fix(pass, file, arg)
	)
	if ok {
		diag.Message += "; write " + text
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Write " + text,
			TextEdits: edits,
		}}
	}
	pass.Report(diag)
}

// fix returns the edits that write context.TODO() in place of arg, and reports
// false when the file cannot name the package or the line has no room.
func fix(pass *analysis.Pass, file *ast.File, arg ast.Expr) (edits []analysis.TextEdit, text string, ok bool) {
	names := imports.Names(pass, file)
	qual, ok := imports.Qualifier(pass, names, "context", arg.Pos())
	if !ok {
		return nil, "", false
	}
	text = qual + ".TODO()"
	if !fits(pass, arg.Pos(), arg.End(), text) {
		return nil, "", false
	}
	edits = []analysis.TextEdit{{
		Pos:     arg.Pos(),
		End:     arg.End(),
		NewText: []byte(text),
	}}
	if _, imported := names["context"]; !imported {
		edits = append(edits, imports.Add(pass, file, "context"))
	}
	return edits, text, true
}

// fits reports whether replacing [from, to) with text keeps the line within
// the width. A line already past the width fits: refusing the fix cannot
// shorten it, and linelen reports the line either way.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	if linelen.Config.Len <= 0 {
		return true
	}
	// Written positions, which a //line directive does not move.
	start, end := pass.Fset.PositionFor(from, false),
		pass.Fset.PositionFor(to, false)
	if start.Line != end.Line {
		return false
	}
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
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
