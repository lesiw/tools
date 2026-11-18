// Package a holds comments that read like Markdown.
package a

import h "net/http"

var _ = h.DefaultClient

// A links to [the docs](foo(bar).
func A() {}

// B links to [the docs](foo\), whose parenthesis is escaped.
func B() {}

// C links to [the docs](). // want "markdown link not supported"
func C() {}

// D links to [the docs](foo(bar)). // want "markdown link not supported"
func D() {}

// E names a member the type does not have, as in [T.Missing](s, sep).
func E() {}

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

type T struct {
	Size int
}

func (T) Count() int { return 0 }

type Box[T any] struct {
	V T
}

func (b *Box[T]) Get() T { return b.V }

// N reports the count, as in [T.Count](1) when unset.
func N() {}

// O reports the size, as in [T.Size](1) when unset.
func O() {}

// P reports the count, as in [T](1) when unset.
func P() {}

// Q reports the digest, as in [crypto.Hash](0) for a default.
func Q() {}

// R reports the value, as in [Box.Get](1) when unset.
func R() {}

// S serves, as in [h.ListenAndServe](addr, nil) when unset.
func S() {}

// U reports the count, as in [T](1) or [Crypto](2) when unset. // want "markdown link not supported"
func U() {}

// V links to [T](https://example.com) for the count.
func V() {}

// W names a member the type does not have, as in [T.Missing](1). // want "markdown link not supported"
func W() {}

// X links to [the docs](<a b>). // want "markdown link not supported"
func X() {}

// Y links to [the docs](<a>b), which has no closing parenthesis.
func Y() {}

// Z links to [the docs](<a\>b>). // want "markdown link not supported"
func Z() {}

// Aa links to [the docs](a\)b), whose parenthesis is escaped. // want "markdown link not supported"
func Aa() {}

// Ab links to [the docs](<>), which has an empty target. // want "markdown link not supported"
func Ab() {}
