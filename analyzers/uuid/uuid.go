// Package uuid reports a UUID built by formatting hex digits into the
// canonical shape.
//
// A UUID is a value with one spelling and one parser, and both live in a
// library. Building the text by hand gets the version and variant nibbles
// wrong, and it produces a string that no parser will agree with.
package uuid

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "uuid",
	Doc:  "reports UUIDs built by hand instead of by a library",
	Run:  run,
}

// uuidFormat matches a format string whose verbs are laid out in the UUID
// shape: five hex groups of 8, 4, 4, 4 and 12 digits, or five groups whose
// widths come from the arguments.
var (
	uuidWide = regexp.MustCompile(`^x{8}-x{4}-x{4}-x{4}-x{12}$`)
	uuidVerb = regexp.MustCompile(`^x-x-x-x-x$`)
	verb     = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z]`)
)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSprintf(pass, call) {
				return true
			}
			format, ok := formatOf(call)
			if !ok {
				return true
			}
			if isUUIDFormat(format) {
				pass.Reportf(call.Pos(), "UUID built by formatting; use a library")
			}
			return true
		})
	}
	return nil, nil
}

// isSprintf reports whether call is a fmt.Sprintf.
func isSprintf(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok {
		return false
	}
	return pkg.Imported().Path() == "fmt"
}

// formatOf returns the format string of a call, and whether it is a literal.
func formatOf(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	format, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return format, true
}

// isUUIDFormat reports whether format lays hex digits out in the shape of a
// UUID.
func isUUIDFormat(format string) bool {
	shape := verb.ReplaceAllString(format, "x")
	return uuidWide.MatchString(shape) || uuidVerb.MatchString(shape)
}
