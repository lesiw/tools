# lesiw.io/tools/analyzers/dupcutset

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupcutset.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupcutset)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `strings` or `bytes` `Trim`, `TrimLeft`,
or `TrimRight` call whose literal cutset repeats a character.

```go
func trim(s string) string {
    return strings.Trim(s, "..") // duplicate '.' in cutset; a cutset is a set of characters
}
```

`strings.Trim` removes the leading and trailing code points contained in the
cutset, and the `TrimLeft` and `TrimRight` docs point at `TrimPrefix` and
`TrimSuffix` for removing one end. A character written twice therefore does not
change what is removed, and the cutset is often a prefix or suffix the author
meant to name.

Only a string literal at the call is checked, because the fix rewrites the
argument in place; parentheses around a literal do not hide it. A cutset held
in a constant or a variable, or written as an expression such as a
concatenation, is left alone, as is a call through a function value. A cutset
that is not valid UTF-8 is also left alone.

The rule is adapted from staticcheck's `SA1024`, which checks the `strings`
Trim family by constant value and reports without a fix. This rule adds the
`bytes` package, checks the literal at the call, and fixes it.

Fixable: the fix writes one copy of each character and leaves the choice
between a cutset and `TrimPrefix` or `TrimSuffix` to the author. An interpreted
literal may be re-quoted, so its escape forms can change, and a raw literal
stays raw.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupcutset/cmd/dupcutset
go tool dupcutset ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupcutset ./...
go tool lesiw.io/tools/cmd/fix -dupcutset ./...
```
