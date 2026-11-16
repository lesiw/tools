# lesiw.io/tools/analyzers/divzero

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/divzero.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/divzero)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports integer division of two integer literals
whose quotient is zero.

```go
func f() int {
    return 1 / 2 // integer division 1 / 2 results in zero
}
```

The spec's Arithmetic operators section
(https://go.dev/ref/spec#Arithmetic_operators) defines integer division with
`x / y` truncated towards zero, so `1 / 2` is 0. The rule reports a division of
two integer literals, with a unary sign or parentheses allowed, and leaves
alone:

- a division with an operand that is not an integer literal, such as a named
  constant, variable, conversion, or call, because the division may be
  deliberate;
- a floating-point division, even when both operands have no fractional part;
- an integer division whose quotient is not zero.

The rule is adapted from staticcheck's SA4025, which reports the same divisions
and offers no fix.

Fixable: the division is replaced with `0`. The fix is withheld where a
comment sits inside the division, because the edit would delete the comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/divzero/cmd/divzero
go tool divzero ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -divzero ./...
go tool lesiw.io/tools/cmd/fix -divzero ./...
```
