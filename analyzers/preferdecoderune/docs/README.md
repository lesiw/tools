# lesiw.io/tools/analyzers/preferdecoderune

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/preferdecoderune.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/preferdecoderune)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a string converted to a rune slice to read
its first rune.

```go
func initial(s string) rune {
	return []rune(s)[0] // string converted to a rune slice; use utf8.DecodeRuneInString
}
```

The conversion decodes every rune in the string to build a slice, then keeps the
first one. `utf8.DecodeRuneInString` decodes one rune. A constant operand
reports as well, though the compiler folds a short one and nothing is left to
save.

The call takes a `string`, so a defined string type or a type parameter needs
`string(s)`, and a defined rune type needs the rune converted back. An operand
written `string(b)` takes `utf8.DecodeRune(b)`, which needs no copy.

The conversion and the call agree on the first rune of every string that is not
empty: an ill-formed byte becomes `utf8.RuneError` in both. The call also
returns the width, which separates an ill-formed byte from an encoded
`utf8.RuneError`.

The rule leaves these shapes alone:

- an index that is not the constant zero, as `[1]` or `[i]`;
- an indexed value that is not a conversion to a rune slice, as `chars(s)[0]`
  or `rs[0]`;
- a conversion of a rune slice, as `[]rune(rs)[0]`, which decodes nothing;
- an argument whose type set is not all strings, as `T ~string | ~[]rune`, of
  which only some members decode;
- a conversion whose element is not a rune type, as `[]byte(s)[0]`;
- a slice taken first, as `[]rune(s)[:1][0]`, which is not an index into the
  conversion;
- a count, as `len([]rune(s))`, which costs no allocation.

Report only: the call returns two values, so a fix would restructure the
enclosing statement, and on an empty string the index panics where the call
returns (`utf8.RuneError`, 0).

The rule is like go-critic's `preferDecodeRune`, which matches the written
`[]rune(s)[0]` with `s` exactly a string. This accepts a defined string type, a
type parameter, a defined rune slice type, and a named constant index as well. Go
issue 45260 is about this shape and closed without a fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/preferdecoderune/cmd/preferdecoderune
go tool preferdecoderune ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -preferdecoderune ./...
```
