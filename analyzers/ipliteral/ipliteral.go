// Package ipliteral provides an analysis.Analyzer that reports IP addresses
// written into source as literals.
//
// An address in a string is an address in one place and a deployment detail
// everywhere else. It cannot be moved to DNS, it cannot fail over, and a
// reader cannot tell a test address from a production one. Loopback and the
// unspecified address are exempt: they are placeholders rather than
// locations, and a listener has to name one.
package ipliteral

import (
	"go/ast"
	"go/token"
	"net"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "ipliteral",
	Doc:  "reports IP addresses written into source as literals",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// A test is a fixture. The rule is about a program naming the
		// place it will talk to.
		name := pass.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(name, "_test.go") {
			continue
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
			if addr, ok := addressIn(value); ok {
				pass.Reportf(lit.Pos(),
					"IP address %s written into source", addr,
				)
			}
			return true
		})
	}
	return nil, nil
}

// addressIn returns the address a string holds, if it holds one worth
// reporting.
//
// Only a host is reported: an address that is part of a URL, of user@host, or
// of host:port. A bare dotted number is left alone because it is as likely to
// be an object identifier, a version, or a mask as it is to be an address, and
// nothing in the text says which.
func addressIn(value string) (string, bool) {
	for _, candidate := range candidates(value) {
		if !candidate.host {
			continue
		}
		addr := net.ParseIP(candidate.text)
		if addr == nil {
			continue
		}
		if addr.IsLoopback() || addr.IsUnspecified() {
			// A placeholder is not a location; keep looking, since
			// the string may name a real host after it.
			continue
		}
		return candidate.text, true
	}
	return "", false
}

// candidates returns the parts of value that could be an address: the whole
// string, a host from a URL, a host from host:port, and a host from
// user@host. Each of those is offered with and without its port.
func candidates(value string) []candidate {
	var (
		out  []candidate
		seen = make(map[string]struct{})
		add  func(s string, host bool)
	)
	add = func(s string, host bool) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, candidate{s, host})
		if h, _, err := net.SplitHostPort(s); err == nil {
			add(h, true)
		}
	}
	add(value, false)
	if _, after, ok := strings.Cut(value, "://"); ok {
		rest := after
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			rest = rest[:j]
		}
		add(rest, true)
	}
	if i := strings.LastIndexByte(value, '@'); i >= 0 {
		add(value[i+1:], true)
	}
	return out
}

// candidate is a piece of a string that could be an address, and whether it
// appears where a host goes.
type candidate struct {
	text string
	host bool
}
