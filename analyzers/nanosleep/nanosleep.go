// Package nanosleep provides an analysis.Analyzer that reports a bare small
// integer passed to time.Sleep.
//
// The time package documents Duration as "an int64 nanosecond count" and
// provides constants such as time.Second, time.Millisecond, and
// time.Nanosecond. A bare integer is therefore read as nanoseconds, so
// time.Sleep(1) sleeps for one nanosecond.
//
// A literal from 1 through 120 is reported. A zero argument does nothing, and
// a larger number may be deliberate; neither is reported. Only a literal is
// reported; a named constant, a conversion to time.Duration, and an expression
// such as n * time.Nanosecond are left alone.
//
// The fix multiplies the literal by the time package's Nanosecond constant,
// under the import name in that file, as in 1 * t.Nanosecond for an import
// named t. The duration is unchanged. With a dot import there is no import
// name to qualify Nanosecond with, so the call is reported without a fix. A
// fix that would take the line past the width is withheld too, and the call is
// still reported.
//
// The rule is inspired by staticcheck's SA1004.
package nanosleep

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/uses"
)

// smallest and largest bound the literals the rule reports. Above largest the
// number is left alone as possibly deliberate, as staticcheck's SA1004 does.
const (
	smallest = 1
	largest  = 120
)

var Analyzer = &analysis.Analyzer{
	Name: "nanosleep",
	Doc:  "reports a bare small integer passed to time.Sleep",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !slept(pass, call) || len(call.Args) != 1 {
				return true
			}
			lit, ok := ast.Unparen(call.Args[0]).(*ast.BasicLit)
			if !ok || lit.Kind != token.INT {
				return true
			}
			value, ok := count(lit)
			if !ok || value < smallest || value > largest {
				return true
			}
			report(pass, call, lit, value)
			return true
		})
	}
	return nil, nil
}

// slept reports whether call resolves to the package-level time.Sleep.
func slept(pass *analysis.Pass, call *ast.CallExpr) bool {
	_, fn, ok := uses.CalleeOf(pass, call.Fun)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "time" &&
		fn.Name() == "Sleep" && fn.Signature().Recv() == nil
}

// count returns lit's value, or false when the value does not fit an int64.
func count(lit *ast.BasicLit) (int64, bool) {
	v := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
	if v.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(v)
}

// report adds a diagnostic at lit and, when the file has an import name for
// the time package, a fix that writes the value with a unit.
func report(pass *analysis.Pass, call *ast.CallExpr, lit *ast.BasicLit, n int64) {
	var (
		qual  = packagename(pass, call)
		diag  = analysis.Diagnostic{Pos: lit.Pos(), End: lit.End()}
		unit  = "nanoseconds"
		value = strconv.FormatInt(n, 10)
	)
	if n == 1 {
		unit = "nanosecond"
	}
	diag.Message = "time.Sleep(" + lit.Value + ") reads the argument as " +
		value + " " + unit
	if qual == "" {
		pass.Report(diag)
		return
	}
	replacement := lit.Value + " * " + qual + ".Nanosecond"
	diag.Message += "; write " + replacement
	if !fits(pass, lit.Pos(), lit.End(), replacement) {
		pass.Report(diag)
		return
	}
	diag.SuggestedFixes = []analysis.SuggestedFix{{
		Message: "Write the duration with a unit",
		TextEdits: []analysis.TextEdit{{
			Pos:     lit.Pos(),
			End:     lit.End(),
			NewText: []byte(replacement),
		}},
	}}
	pass.Report(diag)
}

// packagename returns the import name in the file, or the empty string for a
// dot import.
func packagename(pass *analysis.Pass, call *ast.CallExpr) string {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return ""
	}
	pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
	if !ok {
		return ""
	}
	return pkg.Name()
}

// fits reports whether replacing [from, to) with text keeps the line within
// the width.
func fits(pass *analysis.Pass, from, to token.Pos, text string) bool {
	// Written positions, which a //line directive does not move.
	start, end := pass.Fset.PositionFor(from, false),
		pass.Fset.PositionFor(to, false)
	if start.Line != end.Line {
		return false
	}
	if linelen.Config.Len <= 0 {
		return true
	}
	src, err := pass.ReadFile(start.Filename)
	if err != nil {
		return false
	}
	// A driver that parses other bytes than ReadFile returns puts the
	// position outside the line, so the bounds are checked rather than
	// trusted.
	lines := bytes.Split(src, []byte("\n"))
	if start.Line > len(lines) {
		return false
	}
	line := lines[start.Line-1]
	if start.Column > len(line)+1 || end.Column > len(line)+1 {
		return false
	}
	if !linelen.CheckLine(line) {
		return true
	}
	var rewritten bytes.Buffer
	rewritten.Write(line[:start.Column-1])
	rewritten.WriteString(text)
	rewritten.Write(line[end.Column-1:])
	return linelen.CheckLine(rewritten.Bytes())
}
