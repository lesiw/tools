// Package strconvrange provides an analysis.Analyzer that reports a strconv
// argument outside the documented values.
//
// strconv.ParseInt interprets a string "in the given base (0, 2 to 36)" and
// takes a bit size of 0 to 64; the documentation lists 0, 8, 16, 32, and 64 as
// the sizes of the integer types. The integer formatters take a base of 2 to
// 36. The float functions take a bit size of 32 or 64, the complex functions
// 64 or 128, and the float and complex formatters also take a format byte that
// is one of 'b', 'e', 'E', 'f', 'g', 'G', 'x', and 'X'. The documentation
// lists each set, so a value outside one is a fault.
//
// strconv.ParseFloat uses 64 for a bit size other than 32, and
// strconv.ParseComplex uses 128 for one other than 64. The documentation lists
// only that pair, so any other value is outside it.
//
// Only a constant argument is checked, because the value of a variable is
// unknown. The analyzer reports and offers no fix, because nothing in the call
// indicates which valid value was intended.
//
// The rule is adapted from staticcheck's SA1030.
//
// The README catalogs every check and exemption.
package strconvrange

import (
	"go/ast"
	"go/constant"
	"go/token"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "strconvrange",
	Doc:  "reports a strconv argument outside the documented values",
	Run:  run,
}

// check is one argument of a strconv function and the values its documentation
// lists.
type check struct {
	at      int
	message string
	ok      func(constant.Value) bool
	asRune  bool
}

var rules = map[string][]check{
	"ParseInt": {
		{at: 1, message: "base %s is outside 0, 2 to 36", ok: allowZero},
		{at: 2, message: "bitSize %s is outside 0 to 64", ok: integerBits},
	},
	"ParseUint": {
		{at: 1, message: "base %s is outside 0, 2 to 36", ok: allowZero},
		{at: 2, message: "bitSize %s is outside 0 to 64", ok: integerBits},
	},
	"ParseFloat": {
		{at: 1, message: "bitSize %s is neither 32 nor 64", ok: floatBits},
	},
	"ParseComplex": {
		{at: 1, message: "bitSize %s is neither 64 nor 128", ok: complexBits},
	},
	"FormatInt": {
		{at: 1, message: "base %s is outside 2 to 36", ok: base},
	},
	"FormatUint": {
		{at: 1, message: "base %s is outside 2 to 36", ok: base},
	},
	"FormatFloat": {
		{at: 1, message: "format %s is invalid", ok: format, asRune: true},
		{at: 3, message: "bitSize %s is neither 32 nor 64", ok: floatBits},
	},
	"FormatComplex": {
		{at: 1, message: "format %s is invalid", ok: format, asRune: true},
		{at: 3, message: "bitSize %s is neither 64 nor 128", ok: complexBits},
	},
	"AppendInt": {
		{at: 2, message: "base %s is outside 2 to 36", ok: base},
	},
	"AppendUint": {
		{at: 2, message: "base %s is outside 2 to 36", ok: base},
	},
	"AppendFloat": {
		{at: 2, message: "format %s is invalid", ok: format, asRune: true},
		{at: 4, message: "bitSize %s is neither 32 nor 64", ok: floatBits},
	},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				checkCall(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

func checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	if !ok {
		return
	}
	pkg := fn.Pkg()
	if pkg == nil || pkg.Path() != "strconv" {
		return
	}
	for _, rule := range rules[fn.Name()] {
		if rule.at >= len(call.Args) {
			continue
		}
		var (
			arg       = call.Args[rule.at]
			value, ok = constantValue(pass, arg)
		)
		if !ok || rule.ok(value) {
			continue
		}
		pass.Reportf(arg.Pos(), rule.message, valueText(value, rule.asRune))
	}
}

// constantValue returns the integer value of e when e is constant.
func constantValue(pass *analysis.Pass, e ast.Expr) (constant.Value, bool) {
	tv, ok := pass.TypesInfo.Types[ast.Unparen(e)]
	if !ok || tv.Value == nil {
		return nil, false
	}
	value := constant.ToInt(tv.Value)
	return value, value != nil
}

// valueText writes an argument value for a diagnostic. A format is written as
// the rune it denotes.
func valueText(value constant.Value, asRune bool) string {
	if asRune {
		if n, ok := constant.Int64Val(value); ok {
			return strconv.QuoteRune(rune(n))
		}
	}
	return value.ExactString()
}

// base reports whether v is a base the integer formatters document.
func base(v constant.Value) bool {
	return between(v, 2, 36)
}

// allowZero reports whether v is a base the integer parsers document, which
// adds zero to the formatters' range.
func allowZero(v constant.Value) bool {
	return constant.Sign(v) == 0 || base(v)
}

// integerBits reports whether v is a bit size the integer parsers document.
func integerBits(v constant.Value) bool {
	return between(v, 0, 64)
}

// floatBits reports whether v is a bit size the float functions document.
func floatBits(v constant.Value) bool {
	return equal(v, 32) || equal(v, 64)
}

// complexBits reports whether v is a bit size the complex functions document.
func complexBits(v constant.Value) bool {
	return equal(v, 64) || equal(v, 128)
}

// format reports whether v is a format byte the float and complex formatters
// document.
func format(v constant.Value) bool {
	for _, want := range "beEfgGxX" {
		if equal(v, int64(want)) {
			return true
		}
	}
	return false
}

// between reports whether low <= v <= high.
func between(v constant.Value, low, high int64) bool {
	return constant.Compare(v, token.GEQ, constant.MakeInt64(low)) &&
		constant.Compare(v, token.LEQ, constant.MakeInt64(high))
}

// equal reports whether v is want.
func equal(v constant.Value, want int64) bool {
	return constant.Compare(v, token.EQL, constant.MakeInt64(want))
}
