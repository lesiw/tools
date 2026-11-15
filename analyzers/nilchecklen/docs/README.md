# lesiw.io/tools/analyzers/nilchecklen

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilchecklen.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilchecklen)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a nil check that a len comparison makes
redundant.

```go
func nonEmpty(xs []int) bool {
    return xs != nil && len(xs) > 0 // redundant nil check; len is zero for a nil slice
}
```

The specification's "Length and capacity" section says the length of a nil
slice, map or channel is 0. A nil check joined to a comparison of len with a
constant is redundant when the comparison is false at a length of zero under
`&&`, or true at a length of zero under `||`. The nil check tests `!= nil`
under `&&` and `== nil` under `||`. The constant may be written on either
side, and either operand of the pair may be the nil check.

Only slices, maps and channels qualify; a type parameter qualifies when its
type set holds nothing but those. len of a nil pointer to an array is the
array length, so a nil check on one is not redundant.

The value has to contain no call and no channel receive, so that both reads
see the same value; a conversion is allowed.

The nil check and the comparison have to be adjacent operands of one `&&` or
`||`. An operand between them ends the pair: the rule reads neighbors only,
so a nil check that another operand separates from its comparison is left
alone, even where deleting it would be safe. Parentheses around either
operand are allowed.

The idea comes from staticcheck's `S1009`, which reports the pair only when
the nil check is the left operand of the `&&` or `||` and the len call is the
left operand of its comparison.

Fixable: the nil check, the `&&` or `||` that joins it to the comparison, and
the parentheses around the comparison are deleted. A comment inside the
deleted text withholds the fix, because the comment belongs to the author.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilchecklen/cmd/nilchecklen
go tool nilchecklen ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilchecklen ./...
go tool lesiw.io/tools/cmd/fix -nilchecklen ./...
```
