// Package equalfold provides an analysis.Analyzer that reports a comparison of
// two strings or byte slices converted to the same case.
//
// bytes.ToLower "returns a copy of the byte slice s", and strings.ToLower
// builds a string when a letter changes. A comparison of two such values is
// two copies built for one answer.
//
// A comparison is reported only when both sides call the same function, one of
// ToLower and ToUpper in strings or bytes, and the comparison is written with
// == or !=, or as a bytes.Equal call. One side converted and the other left
// alone is a different question: ToLower(a) == b asks whether a lowers to
// exactly b, and this rule compares two converted values. A switch on a
// converted value is not examined, even where a case converts the same way.
// Only a package-qualified call is examined: a conversion reached through a
// dot import or a function value is not.
//
// Folding and conversion differ for some code points. The strings.EqualFold
// doc says it compares "under simple Unicode case-folding, which is a more
// general form of case-insensitivity", and Go issue 52204 is about the
// difference. strings.ToLower("Σ") is "σ" and strings.ToLower("ς") is "ς", so
// lowering separates a sigma from a final sigma where folding compares them
// equal. strings.ToLower("İ") is "i" and strings.ToUpper("ı") is "I", so
// conversion joins İ to i and ı to I where folding keeps both pairs apart.
//
// Where folding was meant, the copies are not needed: strings.EqualFold takes
// the two originals and allocates nothing, and bytes.EqualFold compares the
// byte slices without copying them.
//
// A hash table keyed by a converted value has to keep the conversion:
// hash/maphash's Hasher doc writes it that way, and says folding is not
// consistent with ToLower for all values.
//
// The rule is adapted from go-critic's equalFold, which reports a comparison
// with only one side converted too.
//
// Report only: the swap changes the result for the code points where folding
// and conversion differ.
package equalfold

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "equalfold",
	Doc:  "reports a comparison of two strings or byte slices converted to the same case",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		checkFile(pass, file)
	}
	return nil, nil
}

type conversion struct {
	qual string // the name the file uses for the package
	fn   string
}

// checkFile reports each comparison and each bytes.Equal over two conversions.
func checkFile(pass *analysis.Pass, file *ast.File) {
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if n.Op != token.EQL && n.Op != token.NEQ {
				return true
			}
			var (
				span, negate = target(stack, n)
				not          = (n.Op == token.NEQ) != negate
			)
			report(pass, span, not, n.X, n.Y)
		case *ast.CallExpr:
			pkg, name, ok := callee(pass, n.Fun)
			if !ok || pkg.Imported().Path() != "bytes" || name != "Equal" {
				return true
			}
			if len(n.Args) != 2 {
				return true
			}
			span, negate := target(stack, n)
			report(pass, span, negate, n.Args[0], n.Args[1])
		}
		return true
	})
}

// target returns the node a report covers and whether a ! outside it negates
// the comparison. A ! is taken with the comparison so the report points at the
// negation.
func target(stack []ast.Node, node ast.Expr) (ast.Expr, bool) {
	var (
		span   = node
		negate bool
	)
	for i := len(stack) - 2; i >= 0; i-- {
		switch parent := stack[i].(type) {
		case *ast.ParenExpr:
			// A ! still covers the comparison when a parenthesis sits
			// between.
		case *ast.UnaryExpr:
			if parent.Op != token.NOT {
				return span, negate
			}
			span, negate = parent, !negate
		default:
			return span, negate
		}
	}
	return span, negate
}

// report reports a comparison of x and y when both sides are converted the
// same way.
func report(pass *analysis.Pass, span ast.Expr, not bool, x, y ast.Expr) {
	left, ok := conversionOf(pass, x)
	if !ok {
		return
	}
	right, ok := conversionOf(pass, y)
	// Both sides have the same type, so the same function name means the same
	// package.
	if !ok || left.fn != right.fn {
		return
	}
	msg := left.qual + "." + left.fn + " on both sides; use "
	if not {
		msg += "!"
	}
	msg += left.qual + ".EqualFold if folding is intended"
	pass.Report(analysis.Diagnostic{
		Pos:     span.Pos(),
		End:     span.End(),
		Message: msg,
	})
}

// conversionOf returns the conversion a call performs, and false for any other
// expression.
func conversionOf(pass *analysis.Pass, e ast.Expr) (conversion, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return conversion{}, false
	}
	pkg, name, ok := callee(pass, call.Fun)
	if !ok || name != "ToLower" && name != "ToUpper" {
		return conversion{}, false
	}
	if path := pkg.Imported().Path(); path != "strings" && path != "bytes" {
		return conversion{}, false
	}
	return conversion{qual: pkg.Name(), fn: name}, true
}

// callee returns the package and the name that a package selector names. A
// dot-imported name is not examined.
func callee(pass *analysis.Pass, fun ast.Expr) (pkg *types.PkgName, name string, ok bool) {
	sel, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, "", false
	}
	pkg, ok = pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok {
		return nil, "", false
	}
	return pkg, sel.Sel.Name, true
}
