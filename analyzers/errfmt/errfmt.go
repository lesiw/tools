// Package errfmt provides an analysis.Analyzer for error format strings:
// error construction routes through fmt.Errorf, and error wrapping routes
// through its %w verb.
//
// fmt.Errorf takes a format string, so the errors.New rewrite turns a percent
// sign in the message into a verb, and the author writes that escape. The
// check reports only a call whose message argument is a string literal: a name
// or a concatenation is not folded into one. It offers no fix, because the
// rewrite can change what the error says. The errors import goes only after
// the file's last call is rewritten.
//
// The %w rewrite edits the verb in place, and is withheld where the format's
// source spelling differs from its value, as with escape sequences.
//
// The README catalogs every check, exemption, and fix.
package errfmt

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/imports"
)

var Analyzer = &analysis.Analyzer{
	Name: "errfmt",
	Doc:  "reports error formatting that bypasses fmt.Errorf or %w",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		droppable := dropsErrors(pass, file)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch callee(pass, call) {
			case "errors.New":
				if droppable {
					pass.Report(analysis.Diagnostic{
						Pos:     call.Pos(),
						End:     call.End(),
						Message: "errors.New can be replaced by fmt.Errorf",
					})
				}
			case "fmt.Errorf":
				checkWrap(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// fmtReaches reports whether fmt reaches the call at pos.
func fmtReaches(pass *analysis.Pass, names map[string]string, pos token.Pos) bool {
	_, reaches := imports.Qualifier(pass, names, "fmt", pos)
	return reaches
}

// dropsErrors reports whether the file drops its errors import when every
// errors.New call becomes fmt.Errorf: every use of errors in it is such a
// call, no declaration shadows fmt at one, and every message is a literal.
func dropsErrors(pass *analysis.Pass, file *ast.File) bool {
	names := imports.Names(pass, file)
	if _, ok := names["errors"]; !ok {
		return false
	}
	if _, ok := names["fmt"]; !ok {
		return false
	}
	// Each call accounts for exactly one use of the package name, so equal
	// counts mean every use of errors in the file is one of these calls.
	var calls, uses, rewritable int
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if callee(pass, n) != "errors.New" {
				return true
			}
			calls++
			if fmtReaches(pass, names, n.Pos()) && messageIsLiteral(n) {
				rewritable++
			}
		case *ast.Ident:
			pkg, ok := pass.TypesInfo.Uses[n].(*types.PkgName)
			if ok && pkg.Imported().Path() == "errors" {
				uses++
			}
		}
		return true
	})
	return rewritable == calls && uses == calls
}

// messageIsLiteral reports whether an errors.New call writes its message as a
// literal.
func messageIsLiteral(call *ast.CallExpr) bool {
	_, ok := ast.Unparen(call.Args[0]).(*ast.BasicLit)
	return ok
}

// callee returns the package-qualified name of a called package
// function.
func callee(pass *analysis.Pass, call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Imported().Path() + "." + sel.Sel.Name
}

func checkWrap(pass *analysis.Pass, call *ast.CallExpr) {
	if len(call.Args) < 2 {
		return
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return
	}
	format, err := strconv.Unquote(lit.Value)
	if err != nil {
		return
	}
	verbs := formatVerbs(format)
	for i, arg := range call.Args[1:] {
		if i >= len(verbs) {
			break
		}
		off := verbs[i]
		if format[off] != 'v' && format[off] != 's' {
			continue
		}
		t := pass.TypesInfo.TypeOf(arg)
		if t == nil || t.String() != "error" {
			continue
		}
		d := analysis.Diagnostic{
			Pos: arg.Pos(),
			End: arg.End(),
			Message: "error can be wrapped with %w, not %" +
				format[off:off+1],
		}
		// The verb offset indexes the unquoted string; the edit is
		// sound only when the source bytes between the delimiters
		// are that string verbatim.
		if lit.Value[1:len(lit.Value)-1] == format {
			pos := lit.Pos() + 1 + token.Pos(off)
			d.SuggestedFixes = []analysis.SuggestedFix{{
				Message: "Wrap the error with %w",
				TextEdits: []analysis.TextEdit{{
					Pos:     pos,
					End:     pos + 1,
					NewText: []byte("w"),
				}},
			}}
		}
		pass.Report(d)
	}
}

// formatVerbs returns the offsets of the argument-consuming verbs
// of format in order, indexing the unquoted string. Flags, width,
// and precision are skipped; %% consumes nothing. Indexed or star
// verbs abort: the mapping to arguments is no longer positional.
func formatVerbs(format string) (verbs []int) {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		for i < len(format) && isFlag(format[i]) {
			i++
		}
		if i >= len(format) {
			break
		}
		switch format[i] {
		case '%':
			continue
		case '*', '[':
			return nil
		}
		verbs = append(verbs, i)
	}
	return verbs
}

// isFlag reports whether b is a format flag, width, or precision
// byte.
func isFlag(b byte) bool {
	return strings.ContainsRune("+-# 0.123456789", rune(b))
}
