// Package dupcutset provides an analysis.Analyzer that reports a strings or
// bytes Trim, TrimLeft, or TrimRight call whose literal cutset repeats a
// character.
//
// strings.Trim removes the leading and trailing code points contained in the
// cutset (https://pkg.go.dev/strings#Trim), and the TrimLeft and TrimRight
// docs point at TrimPrefix and TrimSuffix for removing one end. A character
// written twice therefore does not change what is removed, and the cutset is
// often a prefix or suffix the author meant to name.
//
// Only a string literal at the call is checked, because the fix rewrites the
// argument in place; parentheses around a literal do not hide it. A cutset
// held in a constant or a variable, or written as an expression such as a
// concatenation, is left alone, as is a call through a function value. A
// cutset that is not valid UTF-8 is also left alone. The fix writes one copy
// of each character and leaves the choice between a cutset and TrimPrefix or
// TrimSuffix to the author. An interpreted literal may be re-quoted, so its
// escape forms can change, and a raw literal stays raw.
//
// The rule is adapted from staticcheck's SA1024, which checks the strings Trim
// family by constant value and reports without a fix. This rule adds the
// bytes package and checks the literal at the call.
package dupcutset

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "dupcutset",
	Doc:  "reports a strings or bytes Trim, TrimLeft, or TrimRight call whose literal cutset repeats a character",
	Run:  run,
}

// trims are the standard library functions that take a cutset.
var trims = map[string]struct{}{
	"strings.Trim":      {},
	"strings.TrimLeft":  {},
	"strings.TrimRight": {},
	"bytes.Trim":        {},
	"bytes.TrimLeft":    {},
	"bytes.TrimRight":   {},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if diag, ok := diagnose(pass, call); ok {
				pass.Report(diag)
			}
			return true
		})
	}
	return nil, nil
}

func diagnose(pass *analysis.Pass, call *ast.CallExpr) (analysis.Diagnostic, bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || len(call.Args) != 2 {
		return analysis.Diagnostic{}, false
	}
	if _, ok := trims[fn.FullName()]; !ok {
		return analysis.Diagnostic{}, false
	}
	lit, ok := ast.Unparen(call.Args[1]).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return analysis.Diagnostic{}, false
	}
	value, err := strconv.Unquote(lit.Value)
	// Ranging over an invalid byte yields RuneError, which would make two
	// distinct bytes look like a repeat.
	if err != nil || !utf8.ValidString(value) {
		return analysis.Diagnostic{}, false
	}
	unique, repeated, ok := dedupe(value)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	quoted := strconv.QuoteRune(repeated)
	return analysis.Diagnostic{
		Pos: lit.Pos(),
		End: lit.End(),
		Message: "duplicate " + quoted +
			" in cutset; a cutset is a set of characters",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Keep one copy of each character",
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(requote(lit, unique)),
			}},
		}},
	}, true
}

// dedupe returns value with its repeated characters removed, and the first
// duplicate. A character is a whole rune, so two distinct runes that share a
// byte are not a repeat.
func dedupe(value string) (unique string, repeated rune, ok bool) {
	var (
		b    strings.Builder
		seen = make(map[rune]struct{})
	)
	for _, r := range value {
		if _, dup := seen[r]; dup {
			if !ok {
				repeated, ok = r, true
			}
			continue
		}
		seen[r] = struct{}{}
		b.WriteRune(r)
	}
	return b.String(), repeated, ok
}

// requote writes value as a literal of the same kind as lit. A raw literal
// cannot contain a backtick, so its deduplicated value can stay raw.
func requote(lit *ast.BasicLit, value string) string {
	if strings.HasPrefix(lit.Value, "`") {
		return "`" + value + "`"
	}
	return strconv.Quote(value)
}
