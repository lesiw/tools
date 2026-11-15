# lesiw.io/tools/analyzers/dupsubexpr

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupsubexpr.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupsubexpr)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an operator whose operands are the same value.

```go
func f(items []int, k int) bool {
    return items[k] < items[k] // both operands are the same value
}
```

`items[k] < items[k]` compares one item with itself, so one of the two operands
was meant to name something else.

It reports `==`, `!=`, `<`, `>`, `<=`, `>=`, `-`, `/`, and `%`. `x + x` doubles
a value or repeats a string and `x * x` squares it, so a repeat there is written
on purpose. The bitwise operators and the shifts are out of scope, and the
suite's `bools` pass reports a repeated `&&` or `||`.

A NaN changes the answer of `f == f`, `f != f`, `f <= f`, `f >= f`, `f - f`,
and `f / f`, so an operand that can hold one is left alone: a float or complex,
an interface, a struct or array holding one of those, and a type parameter
whose constraint may admit one. `f < f` and `f > f` are false whether or not a
NaN is involved, so they report, and `%` takes no float at all.

An operand whose second read can differ is left alone: a call can have an
effect, a receive consumes an element, and addressing a composite literal
returns a new value. A conversion, a type assertion, and a builtin that only
reads its arguments give the same value twice, so a repeat can report; the
builtins are `cap`, `complex`, `imag`, `len`, `max`, `min`, and `real`. A
constant expression is left alone unless its value is a bool, wherever it
appears: a size such as `[2 / 2]byte` is written on purpose, while `1 == 1` is
true whatever the code does. A repeat inside a repeat reports once. `v - v`
reports, so a zero of a type parameter is written `var zero T`.

The idea comes from go-critic's dupSubExpr, with a narrower set of operators and
a wider NaN exemption.

Report only: the tool cannot tell which operand was meant.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupsubexpr/cmd/dupsubexpr
go tool dupsubexpr ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupsubexpr ./...
```
