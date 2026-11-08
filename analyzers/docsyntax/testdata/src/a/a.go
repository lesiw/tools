// Package a holds comments that read like Markdown.
package a

// F runs with a `code span` in its doc. // want "code span not supported"
func F() {}

// G is **bold** about it. // want "bold not supported"
func G() {}

// H links to [the docs](https://example.com). // want "markdown link not supported"
func H() {}

// # I is a heading, which Go doc syntax reads as one.
func I() {}

// L fences a block, which Go doc syntax leaves as text. // want "fenced code block not supported"
//
// ```go
// v := 1
// ```
func L() {}

// M fences a block with no language, which reads the same way. // want "fenced code block not supported"
//
// ```
// v := 1
// ```
func M() {}

// J is plain prose about a [Name] and a list:
//
//   - a dash is a list item
//   - so is this one
func J() {}

// K shows a code block, where a backtick belongs:
//
//	v := `raw`
func K() {}
