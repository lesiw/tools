// Package regexpraw provides an analysis.Analyzer that reports a regexp
// pattern written as an interpreted string that doubles every backslash and
// holds at least one.
//
// The spec's string literals (https://go.dev/ref/spec#String_literals) keep a
// backslash as written in a raw string, so a raw \d+ and an interpreted \\d+
// hold the same pattern. The pattern functions of package regexp are checked,
// and the callee comes from the type checker, so an aliased or a dot import is
// covered.
//
// A literal that is already raw is left alone, as is one that holds a
// backtick, which a raw string cannot hold. A literal with an escape other
// than a doubled backslash is left alone too. So is one whose value holds a
// carriage return, because a raw string literal discards carriage returns.
//
// The fix rewrites the literal as a raw string.
//
// The rule is inspired by staticcheck's S1007, which checks regexp.Compile and
// regexp.MustCompile.
package regexpraw

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/uses"
)

var Analyzer = &analysis.Analyzer{
	Name: "regexpraw",
	Doc:  "reports a regexp pattern literal that doubles a backslash",
	Run:  run,
}

// patterns are the regexp functions that take a pattern as their first
// argument.
var patterns = map[string]struct{}{
	"Compile":          {},
	"CompilePOSIX":     {},
	"Match":            {},
	"MatchReader":      {},
	"MatchString":      {},
	"MustCompile":      {},
	"MustCompilePOSIX": {},
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
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "regexp" {
		return
	}
	// Some of these names are also methods on a Regexp value, and their
	// argument is text to match rather than a pattern.
	if fn.Signature().Recv() != nil {
		return
	}
	if _, ok := patterns[fn.Name()]; !ok || len(call.Args) == 0 {
		return
	}
	lit, ok := ast.Unparen(call.Args[0]).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	raw, ok := rawLiteral(lit)
	if !ok {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: "pattern literal has doubled backslashes; use a raw string literal",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Write a raw string",
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(raw),
			}},
		}},
	})
}

// rawLiteral returns the raw string literal that holds the same value as lit,
// and false when lit is not an interpreted literal, holds no doubled
// backslash, or holds a backtick, a carriage return, or an escape other than
// a doubled backslash.
func rawLiteral(lit *ast.BasicLit) (string, bool) {
	source := lit.Value
	if len(source) < 2 || source[0] != '"' {
		return "", false
	}
	if strings.Contains(source, "`") {
		return "", false
	}
	var doubled bool
	for i := 1; i < len(source)-1; {
		if source[i] != '\\' {
			i++
			continue
		}
		if i+1 >= len(source)-1 || source[i+1] != '\\' {
			return "", false
		}
		doubled = true
		i += 2
	}
	if !doubled {
		return "", false
	}
	value, err := strconv.Unquote(source)
	if err != nil || strings.ContainsRune(value, '\r') {
		return "", false
	}
	return "`" + value + "`", true
}
