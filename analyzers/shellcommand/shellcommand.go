// Package shellcommand provides an analysis.Analyzer that reports a call to
// exec.Command or exec.CommandContext whose program name is a constant that
// contains whitespace.
//
// The os/exec package "intentionally does not invoke the system shell." The
// program name is one file name, and every argument after it is handed to the
// program unchanged, so exec.Command("alpha --one") looks up "alpha --one" as
// one name, and the command fails when it runs because no executable has that
// name.
//
// A path may contain a space, and the rule reports it, because the call alone
// cannot separate a path from a shell command line. Only a constant is
// examined, so a name held in a variable is left alone.
//
// The rule is adapted from staticcheck's SA1005, which examines exec.Command,
// leaves a name that holds a path separator alone, and looks only for an ASCII
// space. This analyzer examines exec.CommandContext too, reports a name that
// holds a path separator, and tests for whitespace.
//
// Report only: the call does not say whether the author meant a shell line or
// one file name.
package shellcommand

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "shellcommand",
	Doc:  "reports a program name with whitespace in exec.Command or exec.CommandContext",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			arg, ok := program(pass, call)
			if !ok {
				return true
			}
			name, ok := constantString(pass, arg)
			if !ok || !strings.ContainsFunc(name, unicode.IsSpace) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:     arg.Pos(),
				End:     arg.End(),
				Message: "program name contains whitespace; the whole string is one file name",
			})
			return true
		})
	}
	return nil, nil
}

// program returns the argument that names the program, when call is
// os/exec.Command or os/exec.CommandContext.
func program(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok || pkg.Imported().Path() != "os/exec" {
		return nil, false
	}
	var i int
	switch sel.Sel.Name {
	case "Command":
		i = 0
	case "CommandContext":
		i = 1
	default:
		return nil, false
	}
	// A driver that analyzes a package with errors can reach here.
	if len(call.Args) <= i {
		return nil, false
	}
	return call.Args[i], true
}

// constantString returns the value of e when e is a constant string; the type
// checker folds a literal, a const name and a concatenation into one value.
func constantString(pass *analysis.Pass, e ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}
