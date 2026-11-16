// Package joinseparator provides an analysis.Analyzer that reports a literal
// element of a path.Join or filepath.Join call that ends with a redundant
// separator.
//
// path.Join's doc says "The result is Cleaned", and Clean's doc says a result
// ends with a separator only where it names a root. So a separator at the end
// of an element leaves the string the call returns unchanged, and a value that
// holds only separators may be the root, which is why such a value is not
// reported.
//
// The rule reads only a literal element written at the call, and the suggested
// fix deletes the separators it ends with. A slash separates on every host,
// and filepath takes its separator from the host running the analyzer. On
// Windows a separator can complete a root another argument began, so an
// element is reported only when every argument is a constant and the host's
// Join agrees.
//
// The README catalogs every check, exemption, and fix. The suite writes the
// shape this rule reads: filepathjoin's fix can leave an element ending with
// the separator joinseparator removes.
//
// The rule is adapted from go-critic's filepathJoin, which reports any literal
// argument of filepath.Join that holds a separator; this one reads a trailing
// separator in either join and carries a fix.
package joinseparator

import (
	"go/ast"
	"go/constant"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "joinseparator",
	Doc:  "reports a join element ending in redundant separators",
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

// check reports each literal element of a join call that ends with separators
// the call does not need.
func check(pass *analysis.Pass, call *ast.CallExpr) {
	name, native, ok := join(pass, call)
	if !ok {
		return
	}
	for i, arg := range call.Args {
		lit, ok := ast.Unparen(arg).(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			continue
		}
		trimmed := trim(value, native)
		if trimmed == value || exempt(trimmed, native) {
			continue
		}
		if !redundant(pass, call, i, trimmed, native) {
			continue
		}
		pass.Report(analysis.Diagnostic{
			Pos:            lit.Pos(),
			End:            lit.End(),
			Message:        name + " element ends with a redundant separator",
			SuggestedFixes: fix(lit, value, trimmed),
		})
	}
}

// join returns the name the join function is written as, whether that
// function writes the host's separators, and whether this call is one.
func join(pass *analysis.Pass, call *ast.CallExpr) (name string, native, ok bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return "", false, false
	}
	switch fn.FullName() {
	case "path.Join":
		return "path.Join", false, true
	case "path/filepath.Join":
		return "filepath.Join", true, true
	}
	return "", false, false
}

// trim returns s without the separators it ends with.
func trim(s string, native bool) string {
	for s != "" && separator(s[len(s)-1], native) {
		s = s[:len(s)-1]
	}
	return s
}

// separator reports whether b separates elements of the join.
func separator(b byte, native bool) bool {
	return b == '/' || native && os.IsPathSeparator(b)
}

// exempt reports whether an element trimmed to s must be left alone. A value
// that holds only separators may be the root, because an earlier argument may
// be empty at run time. A volume is a root on Windows: followed by a separator
// it is rooted at the drive, so filepath.Join("C:/", name) is "C:\name" where
// filepath.Join("C:", name) is "C:name".
func exempt(s string, native bool) bool {
	if s == "" {
		return true
	}
	if !native {
		return false
	}
	// VolumeName answers in the host's spelling, so a volume written with a
	// slash is one volume too.
	slash := filepath.FromSlash(s)
	return filepath.VolumeName(slash) == slash
}

// redundant reports whether deleting the separators at the end of argument i
// leaves the path the call names.
//
// On Windows the separator of one element can decide the string an earlier
// element produces: filepath.Join("\\host", "share/") is "\\host\share\",
// because Clean keeps the separator a share root was written with. So the
// answer is read from the call's constants, and an element is left alone where
// they are not all known.
func redundant(pass *analysis.Pass, call *ast.CallExpr, i int, s string, native bool) bool {
	if !native || filepath.Separator != '\\' {
		return true
	}
	values, ok := constants(pass, call)
	if !ok {
		return false
	}
	trimmed := slices.Clone(values)
	trimmed[i] = s
	return filepath.Join(values...) == filepath.Join(trimmed...)
}

// constants returns the values of the call's arguments, and whether every one
// is a constant string.
func constants(pass *analysis.Pass, call *ast.CallExpr) ([]string, bool) {
	values := make([]string, len(call.Args))
	for i, arg := range call.Args {
		v := pass.TypesInfo.Types[arg].Value
		if v == nil || v.Kind() != constant.String {
			return nil, false
		}
		values[i] = constant.StringVal(v)
	}
	return values, true
}

// fix returns the edit that deletes the separators the literal ends with.
func fix(lit *ast.BasicLit, value, trimmed string) []analysis.SuggestedFix {
	// n is the number of bytes the value loses.
	var (
		text = lit.Value
		n    = len(value) - len(trimmed)
	)
	// Cutting the literal leaves its escapes alone, where a quote of the value
	// would rewrite them.
	short := text[:len(text)-1-n] + text[len(text)-1:]
	if v, err := strconv.Unquote(short); err != nil || v != trimmed {
		short = strconv.Quote(trimmed)
	}
	message := "Remove the trailing separator"
	if n > 1 {
		message = "Remove the trailing separators"
	}
	return []analysis.SuggestedFix{{
		Message: message,
		TextEdits: []analysis.TextEdit{{
			Pos:     lit.Pos(),
			End:     lit.End(),
			NewText: []byte(short),
		}},
	}}
}
