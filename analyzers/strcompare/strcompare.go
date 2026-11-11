// Package strcompare provides an analysis.Analyzer that reports a comparison
// made through strings.Compare or bytes.Compare.
//
// strings.Compare documents the rule itself: it is for a three-way
// comparison, and "it is usually clearer and always faster to use the
// built-in string comparison operators ==, <, >, and so on." bytes.Compare
// has no such note, but comparing its result with zero is a longer way to
// write bytes.Equal.
package strcompare

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "strcompare",
	Doc:  "reports a comparison made through strings.Compare or bytes.Compare",
	Run:  run,
}

// calls maps the two functions to whether their result is compared for
// equality only.
var calls = map[string]bool{
	"strings.Compare": false,
	"bytes.Compare":   true,
}

// operators are the comparisons whose answer a Compare call can express.
var operators = map[token.Token]struct{}{
	token.EQL: {},
	token.NEQ: {},
	token.LSS: {},
	token.LEQ: {},
	token.GTR: {},
	token.GEQ: {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			if _, known := operators[bin.Op]; !known {
				return true
			}
			report(pass, bin)
			return true
		})
	}
	return nil, nil
}

// report checks one comparison and reports it when it calls Compare.
func report(pass *analysis.Pass, bin *ast.BinaryExpr) {
	var (
		equalityOnly, ok = callee(pass, bin.X)
		other            = bin.Y
	)
	if !ok {
		equalityOnly, ok = callee(pass, bin.Y)
		other = bin.X
	}
	if !ok || !isConst(other) {
		return
	}
	if equalityOnly && bin.Op != token.EQL && bin.Op != token.NEQ {
		// bytes.Compare has no better form for an ordering.
		return
	}
	pass.Reportf(bin.Pos(), "comparison through Compare; compare directly")
}

// callee reports whether e is a call to one of the two, and whether that
// function has no better form for an ordering.
func callee(pass *analysis.Pass, e ast.Expr) (equalityOnly, ok bool) {
	call, isCall := e.(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return false, false
	}
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel || sel.Sel.Name != "Compare" {
		return false, false
	}
	ident, isIdent := sel.X.(*ast.Ident)
	if !isIdent {
		return false, false
	}
	pkg, isPkg := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !isPkg {
		return false, false
	}
	equalityOnly, ok = calls[pkg.Imported().Path()+".Compare"]
	return equalityOnly, ok
}

// isConst reports whether e is an integer literal, with or without a sign.
func isConst(e ast.Expr) bool {
	if unary, ok := e.(*ast.UnaryExpr); ok {
		if unary.Op != token.SUB && unary.Op != token.ADD {
			return false
		}
		e = unary.X
	}
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.INT
}
