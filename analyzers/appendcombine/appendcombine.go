// Package appendcombine provides an analysis.Analyzer that reports
// consecutive appends to the same slice.
//
// xs = append(xs, 1) followed by xs = append(xs, 2) may grow the slice twice.
// One append takes both values and grows it once. Like go-critic's
// appendCombine.
package appendcombine

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "appendcombine",
	Doc:  "reports consecutive appends to the same slice",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			check(pass, block, file.Comments)
			return true
		})
	}
	return nil, nil
}

// check reports each run of appends to one slice in block.
func check(pass *analysis.Pass, block *ast.BlockStmt, comments []*ast.CommentGroup) {
	var (
		slice       types.Object
		count       int
		first, last token.Pos
	)
	// commented reports whether a comment on its own line sits between the
	// first and last append. Merging the calls would move such a comment
	// away from the line it explains.
	commented := func() bool {
		for _, group := range comments {
			if group.Pos() <= first || group.End() >= last {
				continue
			}
			if standalone(pass, group) {
				return true
			}
		}
		return false
	}
	flush := func() {
		if count > 1 && !commented() {
			pass.Reportf(first, "%d appends to one slice; append the values together", count)
		}
		slice, count = nil, 0
	}
	for _, stmt := range block.List {
		obj, pos, ok := appended(pass, stmt)
		if !ok || (slice != nil && obj != slice) {
			flush()
			if !ok {
				continue
			}
		}
		if count == 0 {
			first = pos
		}
		last = stmt.End()
		slice = obj
		count++
	}
	flush()
}

// standalone reports whether group starts a line.
func standalone(pass *analysis.Pass, group *ast.CommentGroup) bool {
	file := pass.Fset.File(group.Pos())
	if file == nil {
		return false
	}
	src, err := pass.ReadFile(file.Name())
	if err != nil {
		return false
	}
	line := file.Offset(file.LineStart(pass.Fset.Position(group.Pos()).Line))
	off := file.Offset(group.Pos())
	if line < 0 || off > len(src) || line > off {
		return false
	}
	return strings.TrimSpace(string(src[line:off])) == ""
}

// appended returns the slice a statement appends to, and where.
func appended(pass *analysis.Pass, stmt ast.Stmt) (types.Object, token.Pos, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return nil, 0, false
	}
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, 0, false
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, 0, false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return nil, 0, false
	}
	// A spread append cannot join another: append takes one ...
	if call.Ellipsis != token.NoPos {
		return nil, 0, false
	}
	builtin, ok := call.Fun.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[builtin] != types.Universe.Lookup("append") {
		return nil, 0, false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	if !ok || pass.TypesInfo.ObjectOf(arg) != pass.TypesInfo.ObjectOf(lhs) {
		return nil, 0, false
	}
	return pass.TypesInfo.ObjectOf(lhs), assign.Pos(), true
}
