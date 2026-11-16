// Package noopbits provides an analysis.Analyzer that reports a bitwise
// operation with a zero literal or iota zero.
//
// In the spec's operator table, & is bitwise AND, | is bitwise OR, and ^ is
// bitwise XOR (https://go.dev/ref/spec#Arithmetic_operators). A zero bit clears
// the matching result bit under AND, so n & 0 is zero. Under OR and XOR a zero
// bit leaves the matching bit of the other operand, so n | 0 and n ^ 0 are n.
//
// The fix drops the operator and the zero operand. An AND result is not its
// operand, and an iota zero may be a flag meant as 1 << iota
// (https://go.dev/ref/spec#Iota), so neither form is fixed. The README
// catalogs every check, exemption, and fix.
//
// The rule is adapted from staticcheck's SA4016.
package noopbits

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "noopbits",
	Doc:  "reports a bitwise operation with a zero literal or iota zero",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	zeros := iotaZeros(pass)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if bin, ok := n.(*ast.BinaryExpr); ok {
				check(pass, file, zeros, bin)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a bitwise operation whose right operand is a zero constant,
// with a fix where replacing the operation with the other operand keeps its
// value.
func check(pass *analysis.Pass, file *ast.File, zeros map[*types.Const]iotaKind, bin *ast.BinaryExpr) {
	switch bin.Op {
	case token.AND, token.OR, token.XOR:
	default:
		return
	}
	kind, ok := zeroOperand(pass, zeros, bin.Y)
	if !ok || pass.TypesInfo.Types[bin.X].Value != nil {
		return
	}
	message := fmt.Sprintf("%s always equals 0", astexpr.Text(bin))
	if bin.Op != token.AND {
		message = fmt.Sprintf(
			"%s always equals %s",
			astexpr.Text(bin), astexpr.Text(bin.X),
		)
	}
	if kind == iotaDirect {
		message += "; the constant is defined with iota and may be meant as 1 << iota"
	}
	diag := analysis.Diagnostic{
		Pos:     bin.Pos(),
		End:     bin.End(),
		Message: message,
	}
	if bin.Op != token.AND && kind == notIota {
		diag.SuggestedFixes = fixes(file, bin)
	}
	pass.Report(diag)
}

// iotaKind says how a zero operand is an iota constant.
type iotaKind int

const (
	notIota iotaKind = iota
	iotaDerived
	iotaDirect
)

// zeroOperand reports whether e is a zero integer literal or an iota constant
// declared in this package with value zero. The first result says whether the
// constant is iota itself or a zero derived from an iota expression.
func zeroOperand(pass *analysis.Pass, zeros map[*types.Const]iotaKind, e ast.Expr) (iotaKind, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.BasicLit:
		return notIota, e.Kind == token.INT && zeroValue(pass, e)
	case *ast.UnaryExpr:
		if e.Op != token.ADD && e.Op != token.SUB {
			return notIota, false
		}
		lit, ok := ast.Unparen(e.X).(*ast.BasicLit)
		return notIota, ok && lit.Kind == token.INT && zeroValue(pass, e)
	case *ast.Ident:
		obj, ok := pass.TypesInfo.ObjectOf(e).(*types.Const)
		if !ok {
			return notIota, false
		}
		kind, ok := zeros[obj]
		return kind, ok
	}
	return notIota, false
}

// zeroValue reports whether e is an integer constant with value zero.
func zeroValue(pass *analysis.Pass, e ast.Expr) bool {
	v := pass.TypesInfo.Types[e].Value
	return v != nil && v.Kind() == constant.Int && constant.Sign(v) == 0
}

// iotaZeros returns the integer iota constants declared in the package with
// value zero.
func iotaZeros(pass *analysis.Pass) map[*types.Const]iotaKind {
	zeros := make(map[*types.Const]iotaKind)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			gen, ok := n.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				return true
			}
			var values []ast.Expr
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(valueSpec.Values) > 0 {
					values = valueSpec.Values
				}
				for i, name := range valueSpec.Names {
					if i >= len(values) || !containsIota(pass, values[i]) {
						continue
					}
					obj, ok := pass.TypesInfo.Defs[name].(*types.Const)
					if !ok || obj.Val().Kind() != constant.Int {
						continue
					}
					if constant.Sign(obj.Val()) != 0 {
						continue
					}
					kind := iotaDerived
					if directIota(pass, values[i]) {
						kind = iotaDirect
					}
					zeros[obj] = kind
				}
			}
			return true
		})
	}
	return zeros
}

// directIota reports whether e is the predeclared iota, with or without
// parentheses.
func directIota(pass *analysis.Pass, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && predeclaredIota(pass, id)
}

// predeclaredIota reports whether id names the predeclared iota.
func predeclaredIota(pass *analysis.Pass, id *ast.Ident) bool {
	c, ok := pass.TypesInfo.ObjectOf(id).(*types.Const)
	return ok && c.Name() == "iota" && c.Pkg() == nil
}

// containsIota reports whether e uses the predeclared iota.
func containsIota(pass *analysis.Pass, e ast.Expr) bool {
	var found bool
	ast.Inspect(e, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if ok && predeclaredIota(pass, id) {
			found = true
		}
		return !found
	})
	return found
}

// fixes returns the edit that drops the operator and the zero operand, or no
// fix when the edit would delete a comment.
func fixes(file *ast.File, bin *ast.BinaryExpr) []analysis.SuggestedFix {
	lo, hi := bin.X.End(), bin.End()
	if commented(file, lo, hi) {
		return nil
	}
	return []analysis.SuggestedFix{{
		Message: "Replace with the operand",
		TextEdits: []analysis.TextEdit{{
			Pos: lo,
			End: hi,
		}},
	}}
}

// commented reports whether a comment overlaps the span a fix would delete.
func commented(file *ast.File, lo, hi token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < hi && group.End() > lo {
			return true
		}
	}
	return false
}
