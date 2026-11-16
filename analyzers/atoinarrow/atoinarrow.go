// Package atoinarrow provides an analysis.Analyzer that reports a conversion
// of a strconv.Atoi result to int16, or to int32 where int is 64 bits.
//
// Atoi is ParseInt with a base of 10 and a bit size of 0, converted to int,
// and ParseInt reads bit size 0 as the int type
// (https://pkg.go.dev/strconv#Atoi, https://pkg.go.dev/strconv#ParseInt). The
// int type is 32 or 64 bits, so an Atoi result can lie outside int16, and
// outside int32 where int is 64 bits. The conversion discards the high bits;
// no error is reported. ParseInt with a bit size of 16 or 32 returns an error
// for a value outside that type.
//
// Only a call to Atoi is followed; ParseInt with a bit size of 0 returns the
// same range and is left alone. A constant argument is evaluated through
// conversions that keep its underlying type: a value that fits the target is
// left alone, and one outside it is reported. A conversion that changes the
// underlying type and fits the target ends the walk; one that does not fit
// makes the constant unknown, and the conversion to the target type is
// reported.
//
//	func f(s string) int16 {
//		n, _ := strconv.Atoi(s)
//		return int16(n)
//	}
//
// A type whose underlying type is int16 is reported, and int32 where int is 64
// bits; rune is an alias for int32. The rule follows a value through
// assignments, branch merges, loop entries, and numeric conversions within one
// function. An intervening conversion that fits the target type ends the walk.
// It leaves a value that passes through memory or another function alone, and
// it leaves a call through a function value whose target is not known alone. A
// conversion whose result is a type parameter is left alone, and the walk
// stops there.
//
// Report only: ParseInt with a bit size of 16 or 32 returns an error for a
// value that Atoi returns without one, so the repair changes behavior and the
// author must decide how to handle the error.
//
// The rule is adapted from gosec's G109, which follows a variable assigned by
// strconv.Atoi and reports a conversion to the identifiers int16 and int32.
package atoinarrow

import (
	"errors"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name:     "atoinarrow",
	Doc:      "reports a conversion of a strconv.Atoi result to int16, or to int32 where int is 64 bits",
	Run:      run,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	var (
		calls = conversions(pass)
		info  = pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	)
	for _, fn := range sourceFuncs(info) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				convert, ok := instr.(*ssa.Convert)
				if !ok {
					continue
				}
				call, ok := calls[convert.Pos()]
				if !ok {
					continue
				}
				if !fromAtoi(convert.X, convert.Type(), pass.TypesSizes) {
					continue
				}
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(),
					End: call.End(),
					Message: "conversion of a strconv.Atoi result to " +
						astexpr.Text(ast.Unparen(call.Fun)) + " can overflow",
				})
			}
		}
	}
	return nil, nil
}

// sourceFuncs returns the source functions and the package initializer, which
// holds a function literal declared at package level.
func sourceFuncs(info *buildssa.SSA) []*ssa.Function {
	funcs := slices.Clone(info.SrcFuncs)
	if init, ok := info.Pkg.Members["init"].(*ssa.Function); ok {
		funcs = appendAnons(funcs, init)
	}
	return funcs
}

// appendAnons appends fn and its nested function literals.
func appendAnons(funcs []*ssa.Function, fn *ssa.Function) []*ssa.Function {
	funcs = append(funcs, fn)
	for _, anon := range fn.AnonFuncs {
		funcs = appendAnons(funcs, anon)
	}
	return funcs
}

// conversions returns the source conversions to int16 or int32, keyed by the
// left parenthesis that the SSA conversion carries as its position.
func conversions(pass *analysis.Pass) map[token.Pos]*ast.CallExpr {
	out := make(map[token.Pos]*ast.CallExpr)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 || call.Ellipsis != token.NoPos {
				return true
			}
			tv, ok := pass.TypesInfo.Types[call.Fun]
			if !ok || !tv.IsType() || !narrow(pass, tv.Type) {
				return true
			}
			out[call.Lparen] = call
			return true
		})
	}
	return out
}

// narrow reports whether t has int16 or int32 as its underlying type. An int32
// is narrower than int only where int is 64 bits.
func narrow(pass *analysis.Pass, t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	switch b.Kind() {
	case types.Int16:
		return true
	case types.Int32:
		return pass.TypesSizes.Sizeof(types.Typ[types.Int]) > 4
	}
	return false
}

// fromAtoi reports whether v holds the integer result of a strconv.Atoi call
// that can lie outside target.
func fromAtoi(v ssa.Value, target types.Type, sizes types.Sizes) bool {
	return reach(v, target, sizes, true, make(map[visit]bool))
}

// visit names one walk step: a value and whether a conversion can have changed
// a constant that flows through it.
type visit struct {
	value ssa.Value
	exact bool
}

// reach walks v's edges. exact reports whether every conversion between Atoi
// and v keeps the underlying type, so a constant survives unchanged. seen
// holds the values visited, so a value revisited on a cycle ends that branch
// of the walk; another edge can still reach Atoi. An intervening conversion
// that fits target ends the walk, since the value it produces cannot overflow
// target. A conversion whose result is a type parameter ends the walk too.
func reach(v ssa.Value, target types.Type, sizes types.Sizes, exact bool, seen map[visit]bool) bool {
	step := visit{v, exact}
	if seen[step] {
		return false
	}
	seen[step] = true
	switch v := v.(type) {
	case *ssa.Extract:
		call, ok := v.Tuple.(*ssa.Call)
		if !ok || v.Index != 0 {
			return false
		}
		common := call.Common()
		if !atoi(common) {
			return false
		}
		if exact {
			known, overflows := constantOverflows(common, target, sizes)
			if known {
				return overflows
			}
		}
		return true
	case *ssa.Phi:
		for _, edge := range v.Edges {
			if reach(edge, target, sizes, exact, seen) {
				return true
			}
		}
	case *ssa.ChangeType:
		if typeParameter(v.Type()) {
			return false
		}
		return reach(v.X, target, sizes, exact, seen)
	case *ssa.Convert:
		if typeParameter(v.Type()) || fits(v.Type(), target, sizes) {
			return false
		}
		return reach(v.X, target, sizes, false, seen)
	}
	return false
}

// typeParameter reports whether t is a type parameter.
func typeParameter(t types.Type) bool {
	_, ok := t.(*types.TypeParam)
	return ok
}

// fits reports whether every value of type t lies in target's range. Both
// types have an integer underlying type; any other type is left to the walk.
func fits(t, target types.Type, sizes types.Sizes) bool {
	src, ok := t.Underlying().(*types.Basic)
	if !ok || src.Info()&types.IsInteger == 0 {
		return false
	}
	dst, ok := target.Underlying().(*types.Basic)
	if !ok || dst.Info()&types.IsInteger == 0 {
		return false
	}
	srcBits := 8 * sizes.Sizeof(src)
	dstBits := 8 * sizes.Sizeof(dst)
	if src.Info()&types.IsUnsigned != 0 {
		if dst.Info()&types.IsUnsigned != 0 {
			return srcBits <= dstBits
		}
		return srcBits < dstBits
	}
	if dst.Info()&types.IsUnsigned != 0 {
		return false
	}
	return srcBits <= dstBits
}

// constantOverflows reports whether call's argument is a constant and, when
// it is, whether the value lies outside target. A constant with a syntax error
// gives Atoi a result of zero, which fits every target.
func constantOverflows(call *ssa.CallCommon, target types.Type, sizes types.Sizes) (known, overflows bool) {
	if len(call.Args) != 1 {
		return false, false
	}
	arg, ok := call.Args[0].(*ssa.Const)
	if !ok || arg.Value == nil || arg.Value.Kind() != constant.String {
		return false, false
	}
	n, err := strconv.ParseInt(constant.StringVal(arg.Value), 10, 64)
	if err != nil {
		return true, !errors.Is(err, strconv.ErrSyntax)
	}
	return true, !valueFits(n, target, sizes)
}

// valueFits reports whether n lies in target's range.
func valueFits(n int64, target types.Type, sizes types.Sizes) bool {
	b, ok := target.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	bits := 8 * sizes.Sizeof(b)
	if b.Info()&types.IsUnsigned != 0 {
		return n >= 0 && uint64(n) < 1<<bits
	}
	if bits >= 64 {
		return true
	}
	return -int64(1)<<(bits-1) <= n && n <= int64(1)<<(bits-1)-1
}

// atoi reports whether call names strconv.Atoi.
func atoi(call *ssa.CallCommon) bool {
	if call.IsInvoke() {
		return false
	}
	fn := call.StaticCallee()
	if fn == nil {
		return false
	}
	obj, ok := fn.Object().(*types.Func)
	return ok && obj.Pkg() != nil && obj.Pkg().Path() == "strconv" &&
		obj.Name() == "Atoi"
}
