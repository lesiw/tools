// Package source holds the byte-offset helpers that decide whether a range of
// a file occupies whole lines, so that a deleted statement leaves no blank
// line behind.
package source

import "bytes"

// Start returns the offset of the line start at or before off.
func Start(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}

// End returns the offset just past the newline that ends the line at off.
func End(src []byte, off int) int {
	for off < len(src) && src[off] != '\n' {
		off++
	}
	if off < len(src) {
		off++
	}
	return off
}

// Blank reports whether b holds nothing but whitespace.
func Blank(b []byte) bool { return len(bytes.TrimSpace(b)) == 0 }

// Alone reports whether the range from lo to hi is all that its lines hold,
// and whether the last of them ends with a newline.
func Alone(src []byte, lo, hi int) bool {
	first, last := Start(src, lo), End(src, hi)
	if src[last-1] != '\n' {
		return false
	}
	return Blank(src[first:lo]) && Blank(src[hi:last])
}
