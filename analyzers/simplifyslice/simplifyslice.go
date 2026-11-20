// Package simplifyslice provides an analysis.Analyzer that reports a slice
// expression whose high index is the length of the value.
//
// Vendored from gopls' simplifyslice
// (gopls/internal/analysis/simplifyslice, golang.org/x/tools/gopls v0.24.0),
// which is internal and cannot be imported. The rule is upstream's; the code
// is adapted to this repository's helpers.
//
// The slice expressions section of the spec says that "a missing high index
// defaults to the length of the sliced operand", and gives a[2:] as the same
// expression as a[2 : len(a)]
// (https://go.dev/ref/spec#Slice_expressions). The simplify command of gofmt
// writes the short form
// (https://pkg.go.dev/cmd/gofmt#hdr-The_simplify_command); vet and fix do not
// run gofmt -s.
//
// The value and the argument of len are compared by the object each name
// resolves to, and the callee by its own: the call must be to the predeclared
// len. Upstream compares the parser's per-file objects, so this reports a
// value the file does not declare itself, and leaves alone a len declared
// elsewhere in the package, where upstream reports a call to that function.
//
// The fix deletes the high index. It is withheld when the edit would delete a
// comment, where upstream offers the edit unconditionally.
package simplifyslice

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "simplifyslice",
	Doc:  "reports a slice expression whose high index is the length of the value",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if expr, ok := n.(*ast.SliceExpr); ok {
				check(pass, file, expr)
			}
			return true
		})
	}
	return nil, nil
}

// check reports the high index of a two-index slice when it is the length of
// the value being sliced.
func check(pass *analysis.Pass, file *ast.File, expr *ast.SliceExpr) {
	// A three-index slice requires its high and max indices.
	if expr.Max != nil {
		return
	}
	// The rule matches a name against itself, so both sides are bare
	// identifiers.
	value, ok := expr.X.(*ast.Ident)
	if !ok {
		return
	}
	call, ok := expr.High.(*ast.CallExpr)
	if !ok || !isLen(pass, call) {
		return
	}
	arg, ok := call.Args[0].(*ast.Ident)
	if !ok || !astexpr.Same(pass, value, arg) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     expr.High.Pos(),
		End:     expr.High.End(),
		Message: "high index can be omitted; it is the length of the value",
	}
	if !astexpr.Commented(file, expr.High.Pos(), expr.High.End()) {
		diag.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Omit the high index",
			TextEdits: []analysis.TextEdit{{
				Pos: expr.High.Pos(),
				End: expr.High.End(),
			}},
		}}
	}
	pass.Report(diag)
}

// isLen reports whether call is a call to the predeclared len of one argument.
func isLen(pass *analysis.Pass, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	return ok && pass.TypesInfo.Uses[ident] == types.Universe.Lookup("len")
}
