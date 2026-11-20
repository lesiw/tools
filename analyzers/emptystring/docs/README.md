# lesiw.io/tools/analyzers/emptystring

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptystring.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptystring)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a test of whether a string is empty,
written as a comparison of its length with zero.

```go
func f(s string) bool {
    return len(s) == 0 // compare the string with "" instead
}
```

The String types section of the spec says the length of a string is never
negative (https://go.dev/ref/spec#String_types), so `len(s) == 0` and
`len(s) <= 0` both become `s == ""`, and `len(s) != 0` and `len(s) > 0` both
become `s != ""`. A comparison with `""` is the form a reader expects. The
rule matches either operand order.

The zero must be the literal `0`, and `len` must be the builtin. The operand's
type must be a string, or a type whose underlying type is a string, such as
`type word string`; a comparison whose operand is a type parameter is not
reported.

The rule is inspired by go-critic's `emptyStringTest`, which matches the same
four comparisons with the length on the left.

`len(s) < 0` is always false and `len(s) >= 0` is always true.
[`builtinbound`](../../builtinbound/docs) reports these comparisons in either
order and replaces them with the constant.

Fixable: the comparison becomes `s == ""` or `s != ""`. The fix is withheld
when the operand's own text spans more than one line, since the printed form
would put it on one; when the printed form has a newline, as an anonymous struct
type has; and when a comment is inside the comparison, since the rewrite would
drop it. A comparison that wraps while its operand does not still takes the fix.
An operand that holds a composite literal is parenthesized, since the comparison
may sit in a statement header.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptystring/cmd/emptystring
go tool emptystring ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptystring ./...
go tool lesiw.io/tools/cmd/fix -emptystring ./...
```
