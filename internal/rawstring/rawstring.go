// Package rawstring reports whether a string can move into a raw string
// literal unchanged.
package rawstring

import (
	"strings"
	"unicode/utf8"
)

// Holds reports whether s can be written as a raw string literal with the
// same value. A raw literal cannot hold a back quote, the spec drops a
// carriage return from its value, and a NUL or invalid UTF-8 byte is not
// source text.
func Holds(s string) bool {
	if strings.ContainsRune(s, '`') || strings.ContainsRune(s, '\r') {
		return false
	}
	return strings.IndexByte(s, 0) < 0 && utf8.ValidString(s)
}
