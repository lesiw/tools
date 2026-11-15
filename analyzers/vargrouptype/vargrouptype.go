// Package vargrouptype provides an analysis.Analyzer that reports an untyped
// constant whose default type is not the one a spec above it declares.
//
// The Go spec replaces a const spec's omitted expression list with the one
// above it, "and its type if any", so a type carries down only through an
// omission. A variable declared without a type takes the type of its value
// instead, and an untyped constant "is first implicitly converted to its
// default type". A reader of either block expects the specs below a typed spec
// to share its type, and a spec that breaks the expectation holds a value of
// another type without saying so.
//
//	const (
//		first  byte = 1
//		second      = 2
//	)
//
// The README catalogs every check and exemption.
//
// Report only: each repair changes the meaning of the spec, and the tool
// cannot tell which of them the author meant.
//
// The rule is adapted from staticcheck's SA9004, which reads a const block
// whose first spec is typed, reads a literal value only, and adds the type to
// the whole group.
package vargrouptype

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "vargrouptype",
	Doc:  "reports an untyped constant whose default type is not the one declared above it",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if gen, ok := n.(*ast.GenDecl); ok {
				check(pass, gen)
			}
			return true
		})
	}
	return nil, nil
}

// check reports each name in gen whose spec is an untyped constant the type
// above could hold and does not have.
func check(pass *analysis.Pass, gen *ast.GenDecl) {
	if gen.Tok != token.VAR && gen.Tok != token.CONST {
		return
	}
	var (
		declared types.Type
		prev     ast.Spec
	)
	for _, spec := range gen.Specs {
		vs := spec.(*ast.ValueSpec)
		if startsRun(pass.Fset, prev, vs) {
			declared = nil
		}
		prev = spec
		if vs.Type != nil {
			declared = pass.TypesInfo.TypeOf(vs.Type)
			continue
		}
		if len(vs.Values) != len(vs.Names) {
			continue // either an omitted list or one call filling the names
		}
		for i, name := range vs.Names {
			if name.Name != "_" && reportable(pass, vs.Values[i], declared) {
				report(pass, name, declared)
			}
		}
	}
}

// startsRun reports whether spec begins a run of its own: a blank line, or a
// comment on a line of its own, sets it apart from the spec above it. The line
// numbers are the file's own, without //line directives, because the layout is
// what the rule reads.
func startsRun(fset *token.FileSet, prev ast.Spec, spec *ast.ValueSpec) bool {
	if prev == nil {
		return true
	}
	f := fset.File(spec.Pos())
	return f.Line(prev.End())+1 < f.Line(spec.Pos())
}

// reportable reports whether expr is an untyped constant whose default type is
// not declared, and which declared could hold.
func reportable(pass *analysis.Pass, expr ast.Expr, declared types.Type) bool {
	if declared == nil || !untyped(pass.TypesInfo, expr) {
		return false
	}
	if types.Identical(types.Default(pass.TypesInfo.TypeOf(expr)), declared) {
		return false
	}
	return representable(
		pass.TypesInfo.Types[expr].Value,
		declared, pass.TypesSizes,
	)
}

// report reports name, which the declaration gives a type other than declared.
func report(pass *analysis.Pass, name *ast.Ident, declared types.Type) {
	obj := pass.TypesInfo.Defs[name]
	if obj == nil {
		return // an identifier the type checker could not record
	}
	relative := types.RelativeTo(pass.Pkg)
	pass.Reportf(
		name.Pos(), "%s is %s; a spec above declares %s", name.Name,
		types.TypeString(types.Default(obj.Type()), relative),
		types.TypeString(declared, relative),
	)
}

// untyped reports whether expr is spelled as an untyped constant: a literal, a
// constant name, or an operator expression over them. A call such as max(1, 2)
// yields an untyped constant and is not read. A var spec's value carries the
// type the declaration gives it, so only the spelling answers this there.
func untyped(info *types.Info, expr ast.Expr) bool {
	if info.Types[expr].Value == nil {
		return false // not a constant, so it has a type of its own
	}
	switch e := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return untypedConst(info.ObjectOf(e))
	case *ast.SelectorExpr:
		return untypedConst(info.Uses[e.Sel])
	case *ast.UnaryExpr:
		return untyped(info, e.X)
	case *ast.BinaryExpr:
		if e.Op == token.SHL || e.Op == token.SHR {
			return untyped(info, e.X) // a shift takes its type from the left
		}
		return untyped(info, e.X) && untyped(info, e.Y)
	}
	return false
}

// untypedConst reports whether obj is a constant of an untyped type.
func untypedConst(obj types.Object) bool {
	c, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	b, ok := c.Type().(*types.Basic)
	return ok && b.Info()&types.IsUntyped != 0
}

// representable reports whether declared can hold the constant v. A spec that
// named it could not compile with a value it cannot hold, so that type cannot
// be what the author meant.
func representable(v constant.Value, declared types.Type, sizes types.Sizes) bool {
	b, ok := declared.Underlying().(*types.Basic)
	if !ok {
		return false // a type other than a basic one holds no constant
	}
	switch {
	case b.Info()&types.IsBoolean != 0:
		return v.Kind() == constant.Bool
	case b.Info()&types.IsString != 0:
		return v.Kind() == constant.String
	case b.Info()&types.IsInteger != 0:
		lo, hi := intLimits(b, sizes)
		return within(constant.ToInt(v), lo, hi)
	case b.Info()&types.IsFloat != 0:
		return withinFloat(constant.ToFloat(v), b)
	case b.Info()&types.IsComplex != 0:
		c := constant.ToComplex(v)
		return withinFloat(constant.Real(c), b) &&
			withinFloat(constant.Imag(c), b)
	}
	return false
}

// within reports whether x lies between lo and hi. constant.Compare answers
// false for a value of unknown kind, which is what converting another kind of
// constant yields.
func within(x, lo, hi constant.Value) bool {
	return constant.Compare(x, token.GEQ, lo) &&
		constant.Compare(x, token.LEQ, hi)
}

// intLimits returns the least and greatest values the integer kind of b holds.
func intLimits(b *types.Basic, sizes types.Sizes) (lo, hi constant.Value) {
	var (
		bits = uint(8 * sizes.Sizeof(b))
		one  = constant.MakeInt64(1)
	)
	if b.Info()&types.IsUnsigned != 0 {
		top := constant.Shift(one, token.SHL, bits)
		return constant.MakeInt64(0), constant.BinaryOp(top, token.SUB, one)
	}
	top := constant.Shift(one, token.SHL, bits-1)
	hi = constant.BinaryOp(top, token.SUB, one)
	return constant.UnaryOp(token.SUB, top, 0), hi
}

// withinFloat reports whether f rounds to a finite value of the float kind of
// b, which is also the kind of each part of a complex value of b.
func withinFloat(f constant.Value, b *types.Basic) bool {
	if f.Kind() == constant.Unknown {
		return false // another kind of constant converts to nothing
	}
	if b.Kind() == types.Float32 || b.Kind() == types.Complex64 {
		v, _ := constant.Float32Val(f)
		return !math.IsInf(float64(v), 0)
	}
	v, _ := constant.Float64Val(f)
	return !math.IsInf(v, 0)
}
