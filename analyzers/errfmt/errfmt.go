// Package errfmt provides an analysis.Analyzer for error format
// strings: error construction routes through fmt.Errorf, and error
// wrapping routes through its %w verb.
//
// Two checks:
//
//   - errors.New can be replaced by fmt.Errorf. Reported only: a
//     message could contain a percent sign Errorf would interpret,
//     so the rewrite is the author's work.
//   - An error formatted into another error can wrap it: an error
//     value passed to a fmt.Errorf %v or %s verb becomes %w. The
//     fix rewrites the verb in place and is withheld when the
//     format's source spelling differs from its value, as with
//     escape sequences.
//
// Only arguments whose static type is error are considered, and
// only literal format strings are inspected. An indexed or star
// verb skips the whole call: the argument mapping is no longer
// positional.
package errfmt

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "errfmt",
	Doc:  "reports error formatting that bypasses fmt.Errorf or %w",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch callee(pass, call) {
			case "errors.New":
				pass.Report(analysis.Diagnostic{
					Pos:     call.Pos(),
					End:     call.End(),
					Message: "errors.New can be replaced by fmt.Errorf",
				})
			case "fmt.Errorf":
				checkWrap(pass, call)
			}
			return true
		})
	}
	return nil, nil
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
