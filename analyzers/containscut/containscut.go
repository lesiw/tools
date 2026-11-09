// Package containscut reports a Contains check whose answer Cut already
// returns.
//
// Cut reports whether the separator was present, so a Contains over the same
// two operands is a second scan for a boolean the next line throws away. The
// idiomatic form asks the question once.
package containscut

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "containscut",
	Doc:  "reports a Contains check whose answer Cut already returns",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ifStmt, ok := n.(*ast.IfStmt)
			if !ok || ifStmt.Body == nil {
				return true
			}
			s, sep, ok := cutOperands(pass, ifStmt.Cond)
			if !ok {
				return true
			}
			for _, stmt := range ifStmt.Body.List {
				if cutOver(pass, stmt, s, sep) {
					pass.Reportf(ifStmt.Cond.Pos(), "Contains followed by Cut; Cut reports the separator itself")
					return true
				}
			}
			return true
		})
	}
	return nil, nil
}

// cutOperands returns the operands of a strings.Contains call, if the
// condition is one.
func cutOperands(pass *analysis.Pass, cond ast.Expr) (s, sep ast.Expr, ok bool) {
	call, isCall := cond.(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return nil, nil, false
	}
	if !isStringsFunc(pass, call, "Contains") {
		return nil, nil, false
	}
	return call.Args[0], call.Args[1], true
}

// cutOver reports whether stmt holds a strings.Cut over the same two
// operands.
func cutOver(pass *analysis.Pass, stmt ast.Stmt, s, sep ast.Expr) bool {
	var (
		found bool
		visit = func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			if !isStringsFunc(pass, call, "Cut") {
				return true
			}
			if same(pass, call.Args[0], s) && same(pass, call.Args[1], sep) {
				found = true
				return false
			}
			return true
		}
	)
	ast.Inspect(stmt, visit)
	return found
}

// isStringsFunc reports whether call is the named function of package
// strings.
func isStringsFunc(pass *analysis.Pass, call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[ident]
	if !ok {
		return false
	}
	pkg, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}
	return pkg.Imported().Path() == "strings"
}

// same reports whether a and b are written the same way.
func same(pass *analysis.Pass, a, b ast.Expr) bool {
	var (
		objA = pass.TypesInfo.ObjectOf(identOf(a))
		objB = pass.TypesInfo.ObjectOf(identOf(b))
	)
	if objA != nil && objB != nil {
		// A shadowed name is a different variable, however it
		// prints.
		return objA == objB
	}
	var (
		x, y bytes.Buffer
		cfg  = printer.Config{Mode: printer.RawFormat}
	)
	if err := cfg.Fprint(&x, pass.Fset, a); err != nil {
		return false
	}
	if err := cfg.Fprint(&y, pass.Fset, b); err != nil {
		return false
	}
	return x.String() == y.String()
}

// identOf returns the identifier e names, or nil when e is not a plain
// identifier.
func identOf(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}
