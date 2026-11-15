// Package sprintfquotedstring provides an analysis.Analyzer that reports a
// format string that quotes a %s by hand.
//
// %s writes "the uninterpreted bytes of the string or slice", so a quote in
// the value ends the quoted text early and a backslash in it starts an escape.
// %q writes "a double-quoted string safely escaped with Go syntax", quotes
// included.
//
// The fix writes %q in place of the quotes and the verb. The rest of the
// literal keeps its spelling, so a raw literal stays raw.
//
// The README lists the calls and the cases the rule leaves alone.
//
// The rule is adapted from go-critic's sprintfQuotedString, which matches the
// quoted %s in the format's source text, so it also fires where the format
// holds other verbs, and which reports without a fix.
package sprintfquotedstring

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "sprintfquotedstring",
	Doc:  "reports a format string that quotes a %s by hand",
	Run:  run,
}

// formats maps a printf-style function to the index of its format string. A
// method's package is the one that declares it, so the log and testing methods
// match their package's keys.
var formats = map[string]int{
	"fmt.Appendf":    1,
	"fmt.Errorf":     0,
	"fmt.Fprintf":    1,
	"fmt.Printf":     0,
	"fmt.Sprintf":    0,
	"log.Fatalf":     0,
	"log.Panicf":     0,
	"log.Printf":     0,
	"testing.Errorf": 0,
	"testing.Fatalf": 0,
	"testing.Logf":   0,
	"testing.Skipf":  0,
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
	// An interface method such as testing.TB.Errorf has no static callee, so
	// the method object is taken instead.
	fn, ok := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
	if !ok || fn.Pkg() == nil {
		return
	}
	formatIndex, ok := formats[fn.Pkg().Path()+"."+fn.Name()]
	if !ok || len(call.Args) <= formatIndex+1 {
		return
	}
	lit, ok := ast.Unparen(call.Args[formatIndex]).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	format, offsets := unquote(lit.Value)
	open, end, ok := quotedVerb(format)
	if !ok || !quotable(pass.TypesInfo.TypeOf(call.Args[formatIndex+1])) {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: "format string quotes a %s; use %q",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: `Replace "%s" with %q`,
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos() + token.Pos(offsets[open]),
				End:     lit.Pos() + token.Pos(offsets[end]),
				NewText: []byte("%q"),
			}},
		}},
	})
}

// unquote returns the text of a string literal, with its escapes decoded, and,
// for each byte of the text, the offset in the literal's source where it is
// written, then the offset of the closing delimiter. A raw literal's text is
// its source between the delimiters, less a carriage return, which the spec
// discards.
func unquote(source string) (value string, offsets []int) {
	if source[0] == '`' {
		for i := 1; i < len(source)-1; i++ {
			if source[i] == '\r' {
				continue
			}
			offsets = append(offsets, i)
			value += source[i : i+1]
		}
		return value, append(offsets, len(source)-1)
	}
	at := 1
	for rest := source[1:]; rest != "" && rest[0] != '"'; {
		r, multi, tail, err := strconv.UnquoteChar(rest, '"')
		if err != nil {
			return "", nil
		}
		// UnquoteChar returns a byte escape as a rune value, and string(r)
		// would encode that value in UTF-8, so the low byte is taken
		// directly.
		spelled := []byte{byte(r)}
		if multi {
			spelled = []byte(string(r))
		}
		for range spelled {
			offsets = append(offsets, at)
		}
		value += string(spelled)
		at += len(rest) - len(tail)
		rest = tail
	}
	return value, append(offsets, at)
}

// quotedVerb returns the index in format of the opening quote and of the byte
// after the closing quote, when the format's only verb is a bare %s between
// them.
func quotedVerb(format string) (open, end int, ok bool) {
	verb := -1
	for i := 0; i < len(format); {
		if format[i] != '%' {
			i++
			continue
		}
		// A pair of percent signs writes one and starts no verb.
		if i+1 < len(format) && format[i+1] == '%' {
			i += 2
			continue
		}
		if verb >= 0 {
			return 0, 0, false
		}
		verb = i
		i++
	}
	if verb < 1 || verb+2 >= len(format) || !surrounded(format, verb) {
		return 0, 0, false
	}
	// A quote that follows an equals sign closes a field that may escape its
	// own way, and %q escapes for Go.
	if equalsBefore(format, verb-1) {
		return 0, 0, false
	}
	return verb - 1, verb + 3, true
}

// surrounded reports whether the verb at index verb is a bare %s with a double
// quote on each side of it.
func surrounded(format string, verb int) bool {
	if format[verb+1] != 's' {
		return false
	}
	return format[verb-1] == '"' && format[verb+2] == '"'
}

// equalsBefore reports whether the quote at index q follows an equals sign,
// with spaces or tabs in between.
func equalsBefore(format string, q int) bool {
	i := q - 1
	for i >= 0 && (format[i] == ' ' || format[i] == '\t') {
		i--
	}
	return i >= 0 && format[i] == '='
}

// quotable reports whether t is a type %q writes as a quoted string. An
// interface or a type parameter may hold anything.
func quotable(t types.Type) bool {
	if t == nil || hasFormat(t) {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Slice:
		elem, ok := u.Elem().Underlying().(*types.Basic)
		return ok && elem.Kind() == types.Byte
	case *types.Array:
		elem, ok := u.Elem().Underlying().(*types.Basic)
		return ok && elem.Kind() == types.Byte
	}
	return false
}

// hasFormat reports whether t's method set has Format, which is what fmt tests
// for fmt.Formatter.
func hasFormat(t types.Type) bool {
	obj, _, _ := types.LookupFieldOrMethod(t, false, nil, "Format")
	return obj != nil
}
