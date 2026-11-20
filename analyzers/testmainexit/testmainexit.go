// Package testmainexit provides an analysis.Analyzer that reports a TestMain
// function that runs the tests and returns.
//
// The testing package documents that M.Run returns "an exit code that may be
// passed to os.Exit" and that "if TestMain returns, the test wrapper will pass
// the result of M.Run to os.Exit itself" (https://pkg.go.dev/testing#M.Run).
// The go command generates that wrapper from Go 1.15 on, and an earlier one
// exits 0 whatever the tests did, so the rule reads the Go version of the file
// and reports nothing in a file checked against Go 1.15 or later.
//
// The rule reads the function's own body, so an exit inside a helper is not
// seen. Fixable: a body whose only m.Run call is its last statement becomes
// os.Exit(m.Run()), adding an os import to a file that has none.
//
// The rule is adapted from staticcheck's SA3000, which reports "'TestMain'
// doesn't call 'os.Exit', hiding test failures".
//
// The README catalogs every check, exemption, and fix.
package testmainexit

import (
	"go/ast"
	"go/types"
	"go/version"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/imports"
	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "testmainexit",
	Doc:  "reports a TestMain that runs the tests and returns",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		v := pass.TypesInfo.FileVersions[file]
		if v == "" || version.Compare(v, "go1.15") >= 0 {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !testMain(pass, fn) {
				continue
			}
			if d, ok := diagnose(pass, file, fn); ok {
				pass.Report(d)
			}
		}
	}
	return nil, nil
}

// testMain reports whether fn is the TestMain function: no receiver, one
// parameter named with *testing.M.
func testMain(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	if fn.Name.Name != "TestMain" || fn.Recv != nil || fn.Body == nil {
		return false
	}
	params := fn.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 {
		return false
	}
	ptr, ok := pass.TypesInfo.TypeOf(params[0].Type).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil &&
		obj.Pkg().Path() == "testing" && obj.Name() == "M"
}

// diagnose reports a TestMain that runs the tests without exiting, and
// reports false for one that exits or never calls m.Run.
func diagnose(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl) (analysis.Diagnostic, bool) {
	var (
		m     = pass.TypesInfo.ObjectOf(fn.Type.Params.List[0].Names[0])
		runs  []*ast.CallExpr
		exits bool
	)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isExit(pass, call) {
			exits = true
			return false
		}
		if runOf(pass, call, m) {
			runs = append(runs, call)
		}
		return true
	})
	if exits || len(runs) == 0 {
		return analysis.Diagnostic{}, false
	}
	d := analysis.Diagnostic{
		Pos:     runs[0].Pos(),
		End:     runs[0].End(),
		Message: "TestMain does not exit with the code from m.Run",
	}
	if edits, ok := fix(pass, file, fn, runs[0]); ok {
		d.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "Exit with the code from m.Run (changes behavior)",
			TextEdits: edits,
		}}
	}
	return d, true
}

// isExit reports whether call is a call to os.Exit, through any import name.
func isExit(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && fn.Pkg() != nil &&
		fn.Pkg().Path() == "os" && fn.Name() == "Exit"
}

// runOf reports whether call is m.Run on the parameter m.
func runOf(pass *analysis.Pass, call *ast.CallExpr, m types.Object) bool {
	if m == nil || len(call.Args) != 0 {
		return false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" {
		return false
	}
	ident, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return false
	}
	return pass.TypesInfo.ObjectOf(ident) == m
}

// fix returns the edits that exit with the code from run, and reports false
// where the exit would skip code.
func fix(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, run *ast.CallExpr) ([]analysis.TextEdit, bool) {
	if defers(fn.Body) {
		return nil, false
	}
	var (
		last     = fn.Body.List[len(fn.Body.List)-1]
		stmt, ok = last.(*ast.ExprStmt)
	)
	if !ok || ast.Unparen(stmt.X) != run {
		return nil, false
	}
	names := imports.Names(pass, file)
	name, ok := imports.Qualifier(pass, names, "os", run.Pos())
	if !ok {
		return nil, false
	}
	edits := []analysis.TextEdit{
		{Pos: run.Pos(), End: run.Pos(), NewText: []byte(name + ".Exit(")},
		{Pos: run.End(), End: run.End(), NewText: []byte(")")},
	}
	if _, imported := names["os"]; !imported {
		edits = append(edits, imports.Add(pass, file, "os"))
	}
	return edits, true
}

// defers reports whether body defers a call. A function literal defers in its
// own body, which an exit from this one does not skip.
func defers(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			found = true
		}
		return true
	})
	return found
}
