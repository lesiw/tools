# lesiw.io/tools/analyzers/mathintconv

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/mathintconv.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/mathintconv)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a math rounding call on an integer
converted to a float.

```go
func f(n int) float64 {
    return math.Ceil(float64(n)) // math.Ceil on a converted integer returns its argument
}
```

The math package's rounding functions return an integer value. Ceil "returns
the least integer value greater than or equal to x"; Floor returns the greatest
integer value less than or equal to x; Round and RoundToEven return the nearest
integer; and Trunc drops the fractional part. A conversion of an integer to
`float64` produces an integer value already, so each of these calls returns its
argument.

The rule reports a call to `math.Ceil`, `math.Floor`, `math.Round`,
`math.RoundToEven`, or `math.Trunc` when the argument is a conversion to a
floating-point type whose operand has only integers in its type set. The rule
follows a type parameter's constraint to its type set, so a type parameter
constrained by `int`, `~int | ~int8`, or an interface that embeds one is
reported, and a constraint that also holds a float type is left alone. A named
integer type and an alias of one are reported.

A float argument is left alone, since a rounding call can change it. A
conversion whose operand is a float type is left alone too, including a
conversion of an integer nested inside another conversion, as in
`math.Ceil(float64(float32(n)))`. A conversion of an untyped constant is left
alone because type checking gives the operand the float type. A call through a
function value, as in `ceil := math.Ceil; ceil(float64(n))`, is left alone; the
rule reads calls that name `math.Ceil` or one of the other four rounding
functions directly.

Fixable: the fix replaces the call with its argument, which keeps the `float64`
result and the conversion that produces it. `math.Ceil(float64(n))` becomes
`float64(n)`, and `math.Ceil((float64(n)))` becomes `(float64(n))`. The fix is
withheld where a comment sits in the text the edit removes, where the language
requires a call (an expression statement, including one in an init or post
clause, a `go` call, or a `defer` call), and where the call reaches `math`
through a dot import, because goimports removes a named import the edit leaves
unused but keeps a dot import. The diagnostic is still reported.

The rule is adapted from staticcheck's SA4015, which reports `math.Ceil`,
`math.Floor`, `math.Trunc`, `math.IsNaN`, and `math.IsInf`. IsNaN and IsInf
return false for a converted integer, so this rule leaves them alone. It reports
`math.Round` and `math.RoundToEven` too.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/mathintconv/cmd/mathintconv
go tool mathintconv ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -mathintconv ./...
go tool lesiw.io/tools/cmd/fix -mathintconv ./...
```
