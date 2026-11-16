// Package bidicontrol provides an analysis.Analyzer that reports a
// bidirectional control character in a source file.
//
// Go's source is UTF-8 (https://go.dev/ref/spec#Source_code_representation),
// and the unicode package defines the characters the rule reports
// (https://pkg.go.dev/unicode). These characters control the direction of the
// text around them, so the displayed order can differ from the order the
// compiler parses.
//
// A character in a comment or in an interpreted string or rune literal is
// replaced with an escape. A character in a raw string literal is reported
// without a fix, because an escape is literal text there. A character in a
// directive comment is reported without a fix, because the escape would
// change the directive.
//
// The rule is inspired by gosec's G116, which states the rendering hazard.
// The Go documentation above establishes the encoding and the character
// property.
package bidicontrol

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/token"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "bidicontrol",
	Doc:  "reports a bidirectional control character in a source file",
	Run:  run,
}

// span is a comment or a literal in a source file, in byte offsets.
type span struct {
	start, end int
	kind       spanKind
	directive  bool
}

type spanKind int

const (
	spanComment spanKind = iota
	spanInterpreted
	spanRaw
)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		tokFile := pass.Fset.File(file.FileStart)
		src, err := pass.ReadFile(tokFile.Name())
		if err != nil {
			return nil, err
		}
		spans := spansOf(file, tokFile)
		for offset, r := range string(src) {
			if !unicode.Is(unicode.Bidi_Control, r) {
				continue
			}
			var (
				pos = tokFile.Pos(offset)
				end = pos + token.Pos(utf8.RuneLen(r))
				d   = analysis.Diagnostic{
					Pos: pos,
					End: end,
					Message: fmt.Sprintf(
						"bidirectional control character %U",
						r,
					),
				}
			)
			if s, ok := enclosing(spans, offset); ok && fixable(s) {
				d.SuggestedFixes = []analysis.SuggestedFix{{
					Message: "Escape the character",
					TextEdits: []analysis.TextEdit{{
						Pos:     pos,
						End:     end,
						NewText: []byte(escape(r)),
					}},
				}}
			}
			pass.Report(d)
		}
	}
	return nil, nil
}

// spansOf returns the comments and literals of file, sorted by offset.
func spansOf(file *ast.File, tokFile *token.File) (out []span) {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			out = append(out, span{
				start:     tokFile.Offset(comment.Pos()),
				end:       tokFile.Offset(comment.End()),
				kind:      spanComment,
				directive: directive(comment.Text),
			})
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || (lit.Kind != token.STRING && lit.Kind != token.CHAR) {
			return true
		}
		kind := spanInterpreted
		if lit.Value > "" && lit.Value[0] == '`' {
			kind = spanRaw
		}
		out = append(out, span{
			start: tokFile.Offset(lit.Pos()),
			end:   tokFile.Offset(lit.End()),
			kind:  kind,
		})
		return true
	})
	slices.SortFunc(out, func(a, b span) int {
		return cmp.Compare(a.start, b.start)
	})
	return out
}

// enclosing returns the span that holds offset.
func enclosing(spans []span, offset int) (span, bool) {
	i := sort.Search(len(spans), func(i int) bool {
		return spans[i].end > offset
	})
	if i < len(spans) && spans[i].start <= offset {
		return spans[i], true
	}
	return span{}, false
}

func fixable(s span) bool {
	switch s.kind {
	case spanComment:
		return !s.directive
	case spanInterpreted:
		return true
	}
	return false
}

// escape returns the Go escape that names r in a comment or an interpreted
// literal.
func escape(r rune) string {
	return fmt.Sprintf("\\u%04X", r)
}

// directive reports whether text is a comment directive. The colon rule is
// go/ast's isDirective, which is not exported. The bare words are the ones
// commentfmt and commentedoutcode treat as directives. +build, output, and
// line comments are read by the build and example tooling.
func directive(text string) bool {
	if constraint.IsPlusBuild(text) || constraint.IsGoBuild(text) {
		return true
	}
	switch {
	case strings.HasPrefix(text, "//"):
		text = text[2:]
	case strings.HasPrefix(text, "/*"):
		text = strings.TrimSuffix(text[2:], "*/")
	default:
		return false
	}
	if outputDirective(text) || bareDirective(text) {
		return true
	}
	prefixes := [...]string{"line ", "extern ", "export "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return colonDirective(text)
}

func outputDirective(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return strings.HasPrefix(text, "output:") ||
		strings.HasPrefix(text, "unordered output:")
}

func bareDirective(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "nolint", "sys", "sysnb":
		return true
	}
	return false
}

func colonDirective(text string) bool {
	colon := strings.IndexByte(text, ':')
	if colon <= 0 || colon+1 >= len(text) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		b := text[i]
		if !('a' <= b && b <= 'z' || '0' <= b && b <= '9') {
			return false
		}
	}
	return true
}
