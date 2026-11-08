// The package doc and the word table below both name the misspellings the rule
// is about, so this file turns the rule off for itself.
//
//ignore:gospell

// Package gospell provides an analysis.Analyzer that reports a word spelled
// the way Go does not spell it.
//
// Go writes American English. Its own identifiers say so: context.Canceled has
// one l, and the standard library has no "cancelled" anywhere. Prose that
// spells it the other way is a small inconsistency in a package whose code is
// consistent, and a grep for one spelling then misses the other.
//
// The rule covers comments and string literals, which are prose. An
// identifier is left alone: it may be a name this package does not own, and
// renaming it is not a spelling fix.
package gospell

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "gospell",
	Doc:  "reports words spelled the way Go does not spell them",
	Run:  run,
}

// prose maps a spelling Go does not use to the one it does, in prose only.
// "golang" is a host and a path as often as it is a word, so a string literal
// is left alone.
var prose = map[string]string{
	"golang": "Go",
}

// words maps a spelling Go does not use to the one it does. Every entry is a
// word rather than a substring, so a longer word carrying it is unaffected.
var words = map[string]string{
	"analyse":    "analyze",
	"behaviour":  "behavior",
	"cancelled":  "canceled",
	"cancelling": "canceling",
	"centre":     "center",
	"colour":     "color",
	"defence":    "defense",
	"favour":     "favor",
	"flavour":    "flavor",
	"honour":     "honor",
	"humour":     "humor",
	"initialise": "initialize",
	"labour":     "labor",
	"licence":    "license",
	"metre":      "meter",
	"neighbour":  "neighbor",
	"normalise":  "normalize",
	"offence":    "offense",
	"optimise":   "optimize",
	"organise":   "organize",
	"prioritise": "prioritize",
	"recognise":  "recognize",
	"rumour":     "rumor",
	"serialise":  "serialize",
	"summarise":  "summarize",
	"whilst":     "while",
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			if isOutput(group.Text()) {
				continue
			}
			for _, comment := range group.List {
				checkComment(pass, comment)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			checkText(pass, lit.Pos(), value, false, nil)
			return true
		})
	}
	return nil, nil
}

// checkComment reports the misspellings in a comment, skipping the lines that
// are not prose: a directive, and an indented block, which is code.
func checkComment(pass *analysis.Pass, comment *ast.Comment) {
	reported := make(map[string]bool)
	for line := range strings.SplitSeq(comment.Text, "\n") {
		text := strings.TrimPrefix(line, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		if strings.HasPrefix(text, "\t") || strings.HasPrefix(text, "    ") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(text), "go:") {
			continue
		}
		checkText(pass, comment.Pos(), text, true, reported)
	}
}

// isOutput reports whether a comment is an example's expected output.
func isOutput(text string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
		if line == "Output:" || line == "Unordered output:" {
			return true
		}
	}
	return false
}

// checkText reports every misspelling in text. proseOnly words are reported
// only where text is a comment.
func checkText(pass *analysis.Pass, pos token.Pos, text string, isProse bool, reported map[string]bool) {
	for _, word := range split(text) {
		want, ok := words[strings.ToLower(word)]
		if !ok && isProse {
			want, ok = prose[strings.ToLower(word)]
		}
		if !ok {
			continue
		}
		if strings.EqualFold(word, want) {
			continue
		}
		if reported != nil {
			if reported[word] {
				continue
			}
			reported[word] = true
		}
		pass.Reportf(pos, "%s is spelled %s", word, matchCase(word, want))
	}
}

// split returns the words of text that are prose. A word touching a dot, a
// slash, a colon or an at sign is part of a path, a host or an address, where
// the spelling is whatever the other end chose.
func split(text string) []string {
	var (
		out   []string
		start = -1
		flush = func(end int) {
			if start < 0 {
				return
			}
			var before byte
			if start > 0 {
				before = text[start-1]
			}
			// A separator that ends a sentence or a label is
			// punctuation, not path glue: the character after it
			// decides. "behaviour." is a word; "a/b" is a path.
			var after byte
			if end+1 < len(text) {
				after = text[end]
				if !letter(text[end+1]) {
					after = 0
				}
			}
			if !joins(before) && !joins(after) {
				out = append(out, text[start:end])
			}
			start = -1
		}
	)
	for i := 0; i < len(text); i++ {
		if letter(text[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(text))
	return out
}

// letter reports whether b is an ASCII letter.
func letter(b byte) bool { return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' }

// joins reports whether b glues a word to what follows or precedes it.
func joins(b byte) bool {
	return b == '.' || b == '/' || b == ':' || b == '@' || b == '_'
}

// matchCase spells want the way word is capitalized.
func matchCase(word, want string) string {
	if word == strings.ToUpper(word) && len(word) > 1 {
		return strings.ToUpper(want)
	}
	if word != "" && 'A' <= word[0] && word[0] <= 'Z' {
		return strings.ToUpper(want[:1]) + want[1:]
	}
	return want
}
