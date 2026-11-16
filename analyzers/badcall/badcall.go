// Package badcall provides an analysis.Analyzer that reports a call that a
// constant or nil argument leaves with nothing to do.
//
// strings.Replace and bytes.Replace replace "the first n non-overlapping
// instances" of old, so a count of zero replaces nothing. strings.EqualFold
// compares two strings, and bytes.EqualFold two byte slices, under "simple
// Unicode case-folding", which compares rune for rune, and in ASCII a byte is
// a rune. A call on two ASCII constants of different lengths therefore cannot
// match. hash.Hash.Write "adds more data to the running hash", and a nil
// buffer adds nothing.
//
// The Replace check is adapted from go-critic's badCall, which reports the
// zero count of the two SplitN functions too and reads a zero count as a
// mistake for -1. staticcheck's SA1018 reports the count as well. This rule
// examines the Replace count only: a zero count in SplitN returns nil, which
// Go's own examples write to print that result, where a zero count in
// strings.Replace returns its input and in bytes.Replace returns a copy.
//
// Report only: replacing the call with its input, or with false, would keep
// the mistake. A write is left alone too, since the analyzer cannot see which
// Write runs, and removing the call could remove work a wrapper does.
//
// The README catalogs every check and exemption.
package badcall

import (
	"go/ast"
	"go/constant"
	"go/types"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "badcall",
	Doc:  "reports a call that a constant or nil argument leaves with nothing to do",
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

func check(pass *analysis.Pass, call *ast.CallExpr) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok {
		return
	}
	// A method shares its name and package with the function the switch keys
	// on, so (*strings.Replacer).Replace would key as strings.Replace, and a
	// method of a predeclared interface such as error.Error has no package.
	if fn.Signature().Recv() != nil {
		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if ok && isInterfaceWrite(fn) {
			write(pass, call, sel)
		}
		return
	}
	switch fn.Pkg().Path() + "." + fn.Name() {
	case "strings.Replace", "bytes.Replace":
		replace(pass, call)
	case "strings.EqualFold", "bytes.EqualFold":
		equalFold(pass, call)
	}
}

// isInterfaceWrite reports whether the Write a call names is declared by an
// interface. A Write declared on a concrete type is left alone, and a call
// through an interface is reported because the analyzer cannot see which Write
// runs.
func isInterfaceWrite(fn *types.Func) bool {
	return fn.Name() == "Write" &&
		types.IsInterface(fn.Signature().Recv().Type())
}

// replace reports a Replace call whose count replaces nothing.
func replace(pass *analysis.Pass, call *ast.CallExpr) {
	var (
		arg = call.Args[len(call.Args)-1]
		tv  = pass.TypesInfo.Types[arg]
	)
	if tv.Value == nil || tv.Value.Kind() != constant.Int {
		return
	}
	if constant.Sign(tv.Value) != 0 {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     arg.Pos(),
		End:     arg.End(),
		Message: "a count of zero replaces nothing",
	})
}

// equalFold reports an EqualFold call on two ASCII constants of different
// lengths.
func equalFold(pass *analysis.Pass, call *ast.CallExpr) {
	left, ok := constantString(pass, call.Args[0])
	if !ok {
		return
	}
	right, ok := constantString(pass, call.Args[1])
	if !ok || len(left) == len(right) {
		return
	}
	if !isASCII(left) || !isASCII(right) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "the arguments differ in length, so the comparison is always false",
	})
}

// isASCII reports whether s holds no byte above the ASCII range.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

var byteSlice = types.NewSlice(types.Typ[types.Byte])

// constantString returns the constant string e denotes, looking through a
// conversion to a byte slice.
func constantString(pass *analysis.Pass, e ast.Expr) (string, bool) {
	e = ast.Unparen(e)
	tv, ok := pass.TypesInfo.Types[e]
	if ok && tv.Value != nil && tv.Value.Kind() == constant.String {
		return constant.StringVal(tv.Value), true
	}
	// A slice is not constant, so the value of a []byte argument comes from
	// the constant it converts.
	inner, ok := byteSliceOperand(pass, e)
	if !ok {
		return "", false
	}
	return constantString(pass, inner)
}

// byteSliceOperand returns the operand when e converts a value to a byte
// slice.
func byteSliceOperand(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	tv, ok := pass.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return nil, false
	}
	if !types.Identical(tv.Type.Underlying(), byteSlice) {
		return nil, false
	}
	return call.Args[0], true
}

// write reports a nil buffer written to a hash.
func write(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr) {
	// Write takes the buffer as its only argument, so a method expression's
	// receiver, or any other argument list, is not the buffer this reads.
	if len(call.Args) != 1 {
		return
	}
	arg := call.Args[0]
	if !isHash(pass, sel.X) || !isNil(pass, arg) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     arg.Pos(),
		End:     arg.End(),
		Message: "a nil buffer adds nothing to the hash",
	})
}

// isNil reports whether e is nil, looking through a conversion to a byte
// slice.
func isNil(pass *analysis.Pass, e ast.Expr) bool {
	e = ast.Unparen(e)
	if tv, ok := pass.TypesInfo.Types[e]; ok && tv.IsNil() {
		return true
	}
	inner, ok := byteSliceOperand(pass, e)
	return ok && isNil(pass, inner)
}

var hashInterface = newHashInterface()

// isHash reports whether e's type implements hash.Hash.
func isHash(pass *analysis.Pass, e ast.Expr) bool {
	typ := pass.TypesInfo.TypeOf(e)
	if typ == nil {
		return false
	}
	if types.Implements(typ, hashInterface) {
		return true
	}
	// A declared method with a pointer receiver removes the promoted method
	// of the same name from the value's method set, so a value that embeds a
	// hash implements hash.Hash only as a pointer.
	return types.Implements(types.NewPointer(typ), hashInterface)
}

// newHashInterface returns the method set of hash.Hash. It is written out
// because an analyzed package need not import hash, as a package that only
// uses crypto/sha256 does not, and the answer should not depend on an
// unrelated import.
func newHashInterface() *types.Interface {
	var (
		errorType = types.Universe.Lookup("error").Type()
		ints      = []types.Type{types.Typ[types.Int]}
		bytes     = []types.Type{byteSlice}
		writes    = []types.Type{types.Typ[types.Int], errorType}
		iface     = types.NewInterfaceType([]*types.Func{
			types.NewFunc(0, nil, "BlockSize", signature(nil, ints)),
			types.NewFunc(0, nil, "Reset", signature(nil, nil)),
			types.NewFunc(0, nil, "Size", signature(nil, ints)),
			types.NewFunc(0, nil, "Sum", signature(bytes, bytes)),
			types.NewFunc(0, nil, "Write", signature(bytes, writes)),
		}, nil)
	)
	iface.Complete()
	return iface
}

// signature returns the type of a method that takes params and returns
// results. It carries no receiver, which types.Identical and types.Implements
// both ignore, and no package, which types.Identical compares only for an
// unexported name.
func signature(params, results []types.Type) *types.Signature {
	tuple := func(ts []types.Type) *types.Tuple {
		vars := make([]*types.Var, len(ts))
		for i, t := range ts {
			vars[i] = types.NewVar(0, nil, "", t)
		}
		return types.NewTuple(vars...)
	}
	return types.NewSignatureType(
		nil, nil, nil, tuple(params), tuple(results),
		false,
	)
}
