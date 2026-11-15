// Package deepexit provides an analysis.Analyzer that reports a program exit
// outside main or init.
//
// A function that ends the process gives its caller no result and no error.
//
// Report only. The README lists the exemptions and the calls the analyzer does
// not reach.
//
// The idea comes from revive's deep-exit.
package deepexit

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "deepexit",
	Doc:  "reports a program exit outside main or init",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	pkgName := pass.Pkg.Name()
	for _, file := range pass.Files {
		name := pass.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || entrypoint(fn, pkgName) {
				continue
			}
			walk(pass, fn.Body)
		}
	}
	return nil, nil
}

// walk reports the exit calls in body. A literal that the code invokes where
// it is written is walked; one that is kept or passed is not.
func walk(pass *analysis.Pass, body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if lit, ok := ast.Unparen(node.Fun).(*ast.FuncLit); ok {
				walk(pass, lit.Body)
			}
			if name, ok := exitCall(pass, node); ok {
				pass.Reportf(node.Pos(), "%s outside main or init ends the program", name)
			}
		}
		return true
	})
}

// exitCall returns the exit function's name when the call names one. A
// method's package is the one that declares its receiver, so a *log.Logger's
// Fatal matches log.Fatal; every other exit is declared without a receiver.
func exitCall(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok {
		return "", false
	}
	pkg := fn.Pkg()
	if pkg == nil {
		// A method of a universe type belongs to no package; error.Error is
		// the one a program calls.
		return "", false
	}
	name := pkg.Path() + "." + fn.Name()
	switch name {
	case "os.Exit", "syscall.Exit",
		"log.Fatal", "log.Fatalf", "log.Fatalln":
		return name, true
	}
	return "", false
}

// entrypoint reports whether fn is one of the program's entry points: init,
// and the main of the program being built.
func entrypoint(fn *ast.FuncDecl, pkg string) bool {
	if fn.Recv != nil {
		return false
	}
	if fn.Name.Name == "init" {
		return true
	}
	return fn.Name.Name == "main" && pkg == "main"
}
