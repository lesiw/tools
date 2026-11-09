// Package cyclo provides an analysis.Analyzer that reports a function with
// more branches than fit in one reading.
//
// The count is cyclomatic complexity with one change: a switch or a select
// costs one, and its clauses cost nothing. A jump table, a state machine and a
// channel router are dispatch tables, and charging for every arm makes the
// number describe the table rather than the function. Everything else counts
// as it always has: one per if, for and range, and one per && or ||, because
// each of those is a branch a test has to cover.
//
// A function literal is measured on its own and is not charged to the function
// that declares it. A walk defined inside a function is a walk, and a reader
// judges it separately.
//
// The number is a prompt to look at a function, not a verdict on it. What it
// is for is that a reader can see the shape of a function without reading
// every branch, which is the same reason a long function gets split. A finding
// means the function has more branches than fit in one reading; the repair is
// to give the parts names.
package cyclo

import (
	"flag"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// max is the count above which a function is reported. A command sets the
// limit it runs with; the library default is the same one.
var max = 15

// Flags registers the limit.
func Flags(fs *flag.FlagSet) {
	fs.IntVar(&max, "max", max, "maximum cyclomatic complexity")
}

var Analyzer = &analysis.Analyzer{
	Name: "cyclo",
	Doc:  "reports functions whose cyclomatic complexity exceeds the limit",
	Run:  run,
}

func init() { Flags(&Analyzer.Flags) }

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			check(pass, fn.Name.Pos(), fn.Body)
		}
	}
	return nil, nil
}

// check measures body and reports at pos when the count is over the limit. A
// function literal inside body is measured on its own.
func check(pass *analysis.Pass, pos token.Pos, body *ast.BlockStmt) {
	count := 1
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			check(pass, n.Type.Func, n.Body)
			return false
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			count++
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				count++
			}
		}
		return true
	})
	if count <= max {
		return
	}
	pass.Reportf(pos, "cyclomatic complexity %d exceeds %d", count, max)
}
