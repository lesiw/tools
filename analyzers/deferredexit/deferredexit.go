// Package deferredexit provides an analysis.Analyzer that reports an exit
// that happens while deferred calls are pending.
//
// os.Exit and log.Fatal end the process without running deferred calls, so an
// exit after a defer skips it. An exit before the first defer is fine: there
// is nothing pending yet. Like go-critic's exitAfterDefer.
package deferredexit

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "deferredexit",
	Doc:  "reports an exit that happens while deferred calls are pending",
	Run:  run,
}

// exits are the calls that end the process.
var exits = map[string]bool{
	"log.Fatal":   true,
	"log.Fatalf":  true,
	"log.Fatalln": true,
	"os.Exit":     true,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			check(pass, fn.Body)
		}
	}
	return nil, nil
}

// check reports each exit that follows a defer in body.
func check(pass *analysis.Pass, body *ast.BlockStmt) {
	var deferred bool
	for _, stmt := range body.List {
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.DeferStmt:
				deferred = true
			case *ast.CallExpr:
				if deferred && exits[name(pass, n)] {
					pass.Reportf(n.Pos(), "exit skips the deferred calls in this function")
				}
			}
			return true
		})
	}
}

// name returns the name a call's function is written as.
func name(pass *analysis.Pass, call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	pkg, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Imported().Path() + "." + sel.Sel.Name
}
