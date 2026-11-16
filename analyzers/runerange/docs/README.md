# lesiw.io/tools/analyzers/runerange

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/runerange.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/runerange)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range over a string converted to a rune
slice.

```go
func f(s string) {
    for _, r := range []rune(s) { // string converted to a rune slice; range over the string
        use(r)
    }
}
```

The Conversions section of the Go spec says that converting a string to a slice
of runes yields the individual Unicode code points of the string. The range
clause yields the same code points, and decodes one rune per iteration without
building a slice.

The rule is adapted from staticcheck's SA6003.

## Checks

The range expression has to be a conversion, possibly parenthesized, as in
`[]rune(s)`. The operand can be a string or a defined string type, and a type
parameter whose type set holds only strings. The conversion can also name a
defined slice type whose element is `rune`, as in `runes(s)`, or a type
parameter whose type set holds only such slices. The element can be `int32` or
an alias of it. A conversion from a byte slice through a string, as in
`[]rune(string(b))`, reports too.

The key can be absent or written `_`. A constant operand reports as well,
though the compiler folds a short one.

Exempt, deliberately:

- A loop whose key is not `_`, as in `for i, r := range []rune(s)`. A string
  index is the byte offset of the rune, and a rune slice index is the position
  in the slice. The keys differ when a multi-byte rune is followed by another
  rune.
- A conversion stored in a variable before the range, as in `rs := []rune(s)`
  followed by `for _, r := range rs`. The rule reports a conversion written in
  the range expression. SA6003 also reports a conversion stored in a variable
  when only the range uses the variable; this rule leaves that form alone.
- A range over a slice that a call returns, as in `chars(s)`, since the range
  expression is a call.
- A slice whose element is a defined rune type, as in `[]score(s)`. The loop's
  value would have a different type than it has in a range over `[]rune`.
- A slice of the conversion, as in `[]rune(s)[1:]`. The range starts at the
  second rune, where a range over the string starts at the first.
- An operand whose type set holds a type that is not a string, as in
  `~string | ~[]rune`.

## Fixes

The fix deletes the conversion, the call's parentheses, and any parentheses
around the conversion's type. A range over `[]rune(s)` becomes a range over
`s`. It leaves parentheses around the whole conversion or the operand, so
`for _, r := range []rune((s))` becomes `for _, r := range (s)` in the edit.
gofmt removes the redundant parentheses when `cmd/fix` writes the file.

The fix is withheld when a comment sits in either span it deletes; the
diagnostic is still reported. Where the range keyword sits against the
conversion, as in `range[]rune(s)`, the fix writes a space before the operand.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/runerange/cmd/runerange
go tool runerange ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -runerange ./...
go tool lesiw.io/tools/cmd/fix -runerange ./...
```
