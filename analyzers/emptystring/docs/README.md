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
negative, so `len(s) == 0` and `len(s) <= 0` mean the same as `s == ""`, and
`len(s) != 0` and `len(s) > 0` mean the same as `s != ""`. A comparison with
`""` is the form a reader expects. The operand's type must be a string, or a
type whose underlying type is a string, such as `type word string`. A
comparison whose operand is a type parameter is not reported.

`len(s) < 0` is always false and `len(s) >= 0` is always true.
[`lenzero`](../../lenzero/docs) reports such a comparison as the constant it
is.

The rule is inspired by go-critic's `emptyStringTest`, which matches the same
four comparisons with the length on the left. This one matches the zero on
either side.

Fixable: the comparison becomes a comparison with the empty string. The fix is
withheld when a comment is inside the comparison.

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
