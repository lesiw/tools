// Package timelayout provides an analysis.Analyzer that reports a time layout
// that does not spell the reference time: one that names no reference element,
// one that prints the hour of a 12-hour clock with no PM or pm, or one whose
// text does not parse back.
//
// A layout is the reference time, Mon Jan 2 15:04:05 MST 2006, written the way
// the output should read. The time package reads a layout by matching the
// element names its doc lists, and echoes every other character verbatim.
//
// A wrong layout compiles and prints.
//
// The analyzer checks a layout constant passed to Format, AppendFormat, Parse,
// or ParseInLocation, including through a dot import or a method expression.
// The 12-hour clock check runs on Format and AppendFormat, the two that print.
// A layout in a variable is not checked, and neither is a call through a
// function value, a method value, or an interface method.
//
// The parse-back check is adapted from staticcheck's SA1002, which parses a
// layout's own text through time.Parse; this one parses the text the layout
// prints.
//
// No fix is offered. A layout that names no element could become anything, one
// that lacks its PM or pm has two repairs ("3:04" becomes "3:04 PM" or
// "15:04"), and the digits in a layout that does not parse back may be literal
// text.
package timelayout

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"time"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "timelayout",
	Doc:  "reports a time layout that does not spell the reference time",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if arg, prints := layoutArg(pass, call); arg != nil {
				check(pass, arg, prints)
			}
			return true
		})
	}
	return nil, nil
}

// layoutArg returns the argument a call passes as a time layout, or nil when
// the call passes none, and reports whether the call prints a time.
func layoutArg(pass *analysis.Pass, call *ast.CallExpr) (ast.Expr, bool) {
	var (
		callee *ast.Ident
		method bool // a method expression passes the receiver first
	)
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		callee = fun.Sel
		if sel := pass.TypesInfo.Selections[fun]; sel != nil {
			method = sel.Kind() == types.MethodExpr
		}
	case *ast.Ident: // a dot import arrives as a bare identifier
		callee = fun
	default:
		return nil, false
	}
	fn, ok := pass.TypesInfo.Uses[callee].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "time" {
		return nil, false
	}
	arg, prints := 0, true
	switch fn.Name() {
	case "Format":
	case "AppendFormat":
		arg = 1 // the byte slice comes first
	case "Parse", "ParseInLocation":
		prints = false // a mark in the layout would reject the input
	default:
		return nil, false
	}
	if method {
		arg++
	}
	return call.Args[arg], prints
}

// reference is the time every layout is written against, in UTC, and probe is
// an instant whose every element differs from it. A layout that names an
// element prints different text for the two, so equal text means no element
// was named.
var (
	reference = time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC)
	probe     = time.Date(
		2024, time.July, 16,
		8, 9, 10, 123456789,
		time.FixedZone("XYZ", -3*60*60),
	)
)

func check(pass *analysis.Pass, arg ast.Expr, prints bool) {
	tv := pass.TypesInfo.Types[arg]
	if tv.Value == nil {
		return
	}
	layout := constant.StringVal(tv.Value)
	printed := reference.Format(layout)
	if printed == probe.Format(layout) {
		pass.Reportf(arg.Pos(), "layout names no reference element")
	}
	mark := strings.Contains(layout, "PM") || strings.Contains(layout, "pm")
	// Every 3 is the hour of a 12-hour clock, including one the author meant
	// as text: package time reads it that way, and no other element name
	// contains one.
	if prints && strings.Contains(layout, "3") && !mark {
		pass.Reportf(arg.Pos(), "layout prints the hour of a 12-hour clock with no PM or pm")
	}
	if _, err := time.Parse(layout, printed); err != nil {
		pass.Reportf(arg.Pos(), "layout does not parse back the text it prints")
	}
}
