// Package initialisms provides an analysis.Analyzer that reports a name whose
// initialism is written in mixed case.
//
// Go writes an initialism in one case throughout a name: URLPath, urlPath or
// urlpath, never UrlPath. The rule is in the review comments, and a codebase
// that follows it has one form to search for.
package initialisms

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// known are the initialisms the Go review comments name.
var known = []string{
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML",
	"HTTP", "HTTPS", "ID", "IP", "JSON", "QPS", "RAM", "RPC", "SLA", "SMTP",
	"SQL", "SSH", "TCP", "TLS", "TTL", "UDP", "UI", "UID", "UUID", "URI",
	"URL", "UTF8", "VM", "XML", "XMPP", "XSRF", "XSS",
}

var Analyzer = &analysis.Analyzer{
	Name: "initialisms",
	Doc:  "reports an initialism written in mixed case",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || pass.TypesInfo.Defs[id] == nil {
				return true
			}
			if bad, ok := wrong(id.Name); ok {
				pass.Reportf(id.Pos(),
					"%s should be %s in %s", bad, strings.ToUpper(bad),
					id.Name,
				)
			}
			return true
		})
	}
	return nil, nil
}

// wrong returns the first word of name that is an initialism in mixed case.
func wrong(name string) (string, bool) {
	for _, word := range words(name) {
		for _, init := range known {
			if !strings.EqualFold(word, init) {
				continue
			}
			if word == strings.ToUpper(word) || word == strings.ToLower(word) {
				continue
			}
			return word, true
		}
	}
	return "", false
}

// words splits an identifier at its case transitions, so that IsLabel gives Is
// and Label rather than a run of letters that happens to look like one word.
func words(name string) (out []string) {
	var start int
	for i := 1; i < len(name); i++ {
		prev, cur := name[i-1], name[i]
		if !letter(cur) {
			out = append(out, name[start:i])
			start = i
			continue
		}
		if !letter(prev) {
			out = append(out, name[start:i])
			start = i
			continue
		}
		if beginsWord(name, i) {
			out = append(out, name[start:i])
			start = i
		}
	}
	if start < len(name) {
		out = append(out, name[start:])
	}
	return out
}

// beginsWord reports whether the letter at i starts a new word: an upper-case
// letter after a lower-case one, or the last capital of a run before a
// lower-case one.
func beginsWord(name string, i int) bool {
	prev, cur := name[i-1], name[i]
	if !upper(cur) {
		return false
	}
	if lower(prev) {
		return true
	}
	return upper(prev) && i+1 < len(name) && lower(name[i+1])
}

// letter reports whether c is an ASCII letter.
func letter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// upper reports whether c is an ASCII upper-case letter.
func upper(c byte) bool { return 'A' <= c && c <= 'Z' }

// lower reports whether c is an ASCII lower-case letter.
func lower(c byte) bool { return 'a' <= c && c <= 'z' }
