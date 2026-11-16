# lesiw.io/tools/analyzers/atoinarrow

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/atoinarrow.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/atoinarrow)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a conversion of a `strconv.Atoi` result to
`int16`, or to `int32` where `int` is 64 bits.

```go
func f(s string) int16 {
    n, _ := strconv.Atoi(s)
    return int16(n) // the conversion can overflow
}
```

`strconv.Atoi` is `ParseInt` with a base of 10 and a bit size of 0, converted
to `int`, and `ParseInt` reads bit size 0 as `int`
(https://pkg.go.dev/strconv#Atoi, https://pkg.go.dev/strconv#ParseInt). `int`
is 32 or 64 bits, so an `Atoi` result can lie outside `int16`, and outside
`int32` where `int` is 64 bits. The conversion discards the high bits; no error
is reported. `ParseInt` with a bit size of 16 or 32 returns an error for a
value outside that type. Only a call to `Atoi` is followed; `ParseInt` with a
bit size of 0 returns the same range and is left alone. A constant argument is
evaluated through conversions that keep its underlying type: a value that fits
the target is left alone, and one outside it is reported. A conversion that
changes the underlying type and fits the target ends the walk; one that does
not fit makes the constant unknown, and the conversion to the target type is
reported.

A type whose underlying type is `int16` is reported, and a type whose
underlying type is `int32` is reported where `int` is 64 bits; `rune` is an
alias for `int32`. The rule follows a value through assignments, branch merges,
loop entries, and numeric conversions within one function. It reports the
conversion when any path the walk follows reaches `Atoi`. An intervening
conversion that fits the target type ends the walk. The rule does not follow a
bounds check, so a conversion after an explicit range test is still reported;
`//ignore:atoinarrow` suppresses it.

The rule does not report a conversion to another type on its own, including
`int8`, `uint16`, `uint32`, and `int64`. It leaves a value alone when the value
reaches the conversion only through memory (a variable whose address is taken,
a field, an element, or a package variable), another function, an arithmetic
operation, an interface conversion, a type assertion, or a channel. It leaves
a call through a function value whose target is not known alone. A conversion
whose result is a type parameter is left alone, even when every type in its
type set is `int16` or `int32`, and the walk stops there.

Report only: `ParseInt` with a bit size of 16 or 32 returns an error for a
value that `Atoi` returns without one, so the repair changes behavior and the
author must decide how to handle the error.

The rule is adapted from gosec's G109, which follows a variable assigned by
`strconv.Atoi` and reports a conversion to the identifiers `int16` or `int32`.
This rule follows the value through local flow and reports named types as
well.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/atoinarrow/cmd/atoinarrow
go tool atoinarrow ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -atoinarrow ./...
```
