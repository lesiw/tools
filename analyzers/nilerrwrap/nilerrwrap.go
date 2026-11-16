// Package nilerrwrap provides an analysis.Analyzer that reports a fmt.Errorf
// call whose %w verb has no error to wrap.
//
// The errors package documentation states that a nil error returned from
// Unwrap means the error wraps nothing. The fmt.Errorf documentation says it
// is invalid to supply %w with an operand that does not implement error. A
// call with a missing argument, an argument that is not an error, or a nil
// error does not wrap a cause: fmt writes a %!w(...) marker for the operand or
// %!w(MISSING) for a missing one.
//
// The rule is inspired by the x/tools printf analyzer, which reports the
// missing and non-error operands; this rule adds the nil error. The overlap is
// intentional: a run of this analyzer alone covers all three cases. In the
// suite, printf reports the missing and non-error operands too, so a call that
// both analyzers report has two diagnostics. The format string has to be
// constant, so the verbs are known.
//
// A call with a spread argument is left alone, because the argument at a %w
// position is not known. A call with an indexed argument or a star width or
// precision is left alone, because the analyzer does not resolve an explicit
// index or a star argument. A nil literal and a conversion of nil to an
// interface type are reported; any other expression is not inspected for
// nilness. An interface-typed argument is left alone except for a conversion
// from nil or from a concrete type that does not implement error; the dynamic
// type of any other interface value is not known. A type parameter is left
// alone; the analyzer does not inspect its type set. A call through a function
// value or a wrapper is left alone, because the callee expression is not
// fmt.Errorf. Report only: the tool cannot invent the error the author meant
// to wrap.
package nilerrwrap

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "nilerrwrap",
	Doc:  "reports a fmt.Errorf %w verb with no error to wrap",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

// check reports a %w verb in a fmt.Errorf call whose argument is missing, is
// not an error, or is nil.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	if !errorf(pass, call.Fun) || len(call.Args) == 0 {
		return
	}
	if call.Ellipsis.IsValid() {
		return
	}
	format, ok := constantString(pass, call.Args[0])
	if !ok {
		return
	}
	verbs := formatVerbs(format)
	args := call.Args[1:]
	for i, verb := range verbs {
		if verb != 'w' {
			continue
		}
		if i >= len(args) {
			pass.Reportf(call.Pos(), "format has %%w but no error argument")
			return
		}
		arg := args[i]
		if isNil(pass, arg) {
			pass.Reportf(arg.Pos(), "%%w operand is nil")
			return
		}
		t := pass.TypesInfo.TypeOf(arg)
		if _, isParam := t.(*types.TypeParam); isParam {
			continue
		}
		switch {
		case isError(t):
		case !isInterface(t):
			pass.Reportf(arg.Pos(), "%%w argument is not an error")
			return
		default:
			operand, ok := conversion(pass, arg)
			if !ok {
				continue
			}
			ot := pass.TypesInfo.TypeOf(operand)
			if ot != nil && !isInterface(ot) && !isError(ot) {
				pass.Reportf(arg.Pos(), "%%w argument is not an error")
				return
			}
		}
	}
}

// errorf reports whether fun is fmt.Errorf.
func errorf(pass *analysis.Pass, fun ast.Expr) bool {
	var obj types.Object
	switch fun := ast.Unparen(fun).(type) {
	case *ast.SelectorExpr:
		obj = pass.TypesInfo.Uses[fun.Sel]
	case *ast.Ident:
		obj = pass.TypesInfo.Uses[fun]
	}
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "fmt" {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() == nil && fn.Name() == "Errorf"
}

// constantString returns the value of a constant string argument.
func constantString(pass *analysis.Pass, arg ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[arg]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// formatVerbs returns the verbs of format in argument order. Flags and literal
// width or precision are skipped; %% consumes nothing. An indexed argument or
// a star width or precision returns nil: the analyzer does not resolve an
// explicit index or a star argument.
func formatVerbs(format string) (verbs []byte) {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		for i < len(format) && isFlag(format[i]) {
			i++
		}
		if i >= len(format) {
			break
		}
		switch format[i] {
		case '%':
			continue
		case '*', '[':
			return nil
		}
		verbs = append(verbs, format[i])
	}
	return verbs
}

// isFlag reports whether b is a format flag or a literal width or precision
// byte.
func isFlag(b byte) bool {
	return strings.ContainsRune("+-# 0.123456789", rune(b))
}

// conversion returns the operand of a type conversion.
func conversion(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, false
	}
	return call.Args[0], true
}

// isNil reports whether e is nil at compile time. A conversion of nil to an
// interface type passes the nil through; a conversion of nil to any other type
// produces a typed nil, which is not a nil error.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return pass.TypesInfo.Uses[e] == types.Universe.Lookup("nil")
	case *ast.CallExpr:
		operand, ok := conversion(pass, e)
		if !ok {
			return false
		}
		t := pass.TypesInfo.TypeOf(e)
		if _, isParam := t.(*types.TypeParam); isParam {
			return false
		}
		if !isInterface(t) {
			return false
		}
		return isNil(pass, operand)
	}
	return false
}

// isInterface reports whether t is an interface type.
func isInterface(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// isError reports whether t implements the error interface.
func isError(t types.Type) bool {
	if t == nil {
		return false
	}
	var (
		errType   = types.Universe.Lookup("error").Type()
		iface, ok = errType.Underlying().(*types.Interface)
	)
	return ok && types.Implements(t, iface)
}
