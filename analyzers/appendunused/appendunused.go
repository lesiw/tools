// Package appendunused provides an analysis.Analyzer that reports an append
// result assigned to the blank identifier.
//
// The "Appending to and copying slices" section of the spec says that append
// returns the updated slice
// (https://go.dev/ref/spec#Appending_and_copying_slices). A slice header is
// passed by value, so a call does not change the slice variable passed to it:
// that variable keeps its old length, and the returned header has the new
// length.
// Writing the result to the blank identifier discards that header. Another
// slice may share the backing array, so writes into spare capacity can be
// visible through that slice. The rule reports the assignment that drops the
// returned header.
//
// The "Expression statements" section does not permit append in statement
// context (https://go.dev/ref/spec#Expression_statements), so the form that
// discards the result is _ = append(xs, v). A blank target of a multi-value
// assignment or short declaration is reported too.
//
// An append with no values after the slice is not reported; the x/tools
// appends pass reports it in the suite. A call whose first argument is not an
// identifier is not reported either; the rule matches the identifier form
// only. A slice expression or a field may be a buffer the author is filling on
// purpose.
//
// Report only: assigning the result to the slice and removing the append are
// both plausible repairs, and neither edit is safe without knowing the intent.
//
// The rule is adapted from staticcheck's SA4010, which reports an append
// result that is never used. SA4010 also checks that the backing array is not
// aliased; this rule reports the blank assignment without that check.
package appendunused

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "appendunused",
	Doc:  "reports an append result assigned to the blank identifier",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if assign, ok := n.(*ast.AssignStmt); ok {
				check(pass, assign)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each target of assign that writes an append result to the
// blank identifier.
func check(pass *analysis.Pass, assign *ast.AssignStmt) {
	if assign.Tok != token.ASSIGN && assign.Tok != token.DEFINE {
		return
	}
	if len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for i, lhs := range assign.Lhs {
		blank, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok || blank.Name != "_" {
			continue
		}
		call, ok := ast.Unparen(assign.Rhs[i]).(*ast.CallExpr)
		if !ok {
			continue
		}
		builtin, ok := ast.Unparen(call.Fun).(*ast.Ident)
		if !ok {
			continue
		}
		if pass.TypesInfo.Uses[builtin] != types.Universe.Lookup("append") {
			continue
		}
		if len(call.Args) < 2 {
			continue
		}
		slice, ok := ast.Unparen(call.Args[0]).(*ast.Ident)
		if !ok {
			continue
		}
		pass.Reportf(
			call.Pos(),
			"append to %s discards its result, so %s keeps its length",
			slice.Name, slice.Name,
		)
	}
}
