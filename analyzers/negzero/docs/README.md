# lesiw.io/tools/analyzers/negzero

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/negzero.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/negzero)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a unary minus on a floating-point or
complex constant zero.

```go
func f() float64 {
    return -0.0 // a constant cannot be negative zero; this is zero
}
```

The Constants section of the spec says numeric constants "represent exact
values of arbitrary precision and do not overflow. Consequently, there are no
constants denoting the IEEE 754 negative zero, infinity, and not-a-number
values" (https://go.dev/ref/spec#Constants). So `-0.0` is the constant zero,
and the minus is not part of its value. The representability rules say the
same: rounding a constant to a floating-point type simplifies an IEEE negative
zero "to an unsigned zero", and "constant values never result in an IEEE
negative zero, NaN, or infinity"
(https://go.dev/ref/spec#Representability).

The rule reports the minus when the operand's exact value is zero and the
expression's type is floating-point or complex. The type may be untyped,
typed, supplied by the context, or a type parameter restricted to those types.
It covers `-0.0`, the other float literal forms such as `-0e0` and `-0x0p0`, a
typed constant such as `-typedZero`, a named constant such as `-zero`, and a
complex zero such as `-0i`. The expression has the type of its context, so an
integer zero that a float or complex context converts is reported as well:
`float64(-0)`, `var f float64 = -0`, and an argument to a function that takes a
float. An explicit conversion of a typed integer constant zero such as
`float64(-typedIntZero)` is reported too. An integer zero on its own is left
alone: integer types have no negative zero. A tiny nonzero constant that
rounds to zero in the context is left alone, since the constant itself is not
zero.

The rule leaves alone a value that is not constant. A minus on a variable can
produce negative zero at run time, which is why it is not reported. A type
parameter is read through its type set: a parameter whose set holds only
floating-point or complex types is reported, and one whose set holds another
type is left alone.

Negative zero exists at run time. `math.Copysign(0, -1)` returns a
floating-point negative zero, and a complex negative zero is written with one
call per component.

Fixable: the minus is removed; the value is unchanged. No fix is offered when
the minus and the operand are on different lines, since the line break can end
the statement once the minus is gone. An author who wanted negative zero needs
the `math.Copysign` form; the analyzer does not write it, because the value
would change.

The rule is adapted from staticcheck's SA4026.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/negzero/cmd/negzero
go tool negzero ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -negzero ./...
go tool lesiw.io/tools/cmd/fix -negzero ./...
```
