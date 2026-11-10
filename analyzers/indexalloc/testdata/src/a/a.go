package a

import "strings"

func f(b []byte, s string) int {
	return strings.Index(string(b), s) // want `string conversion only to search; use bytes.Index`
}

func ok(b []byte, s string) int {
	return strings.Index(s, "x")
}

func runes(r []rune, s string) int {
	return strings.Index(string(r), s)
}

func normal(s, sub string) int {
	return strings.Index(s, sub)
}
