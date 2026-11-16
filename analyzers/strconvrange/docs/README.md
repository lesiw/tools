# lesiw.io/tools/analyzers/strconvrange

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/strconvrange.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/strconvrange)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a strconv base, bit size, or format
argument outside the documented values.

```go
func f(s string) (int64, error) {
    return strconv.ParseInt(s, 1, 64) // base 1 is outside 0, 2 to 36
}
```

The strconv documentation lists the values for each function.
`strconv.ParseInt` and `strconv.ParseUint` take a base of 0 or 2 to 36 and a
bit size of 0 to 64. `strconv.FormatInt`, `strconv.FormatUint`,
`strconv.AppendInt`, and `strconv.AppendUint` take a base of 2 to 36.
`strconv.ParseFloat`, `strconv.FormatFloat`, and `strconv.AppendFloat` take a
bit size of 32 or 64. `strconv.ParseComplex` and `strconv.FormatComplex` take a
bit size of 64 or 128. `strconv.FormatFloat`, `strconv.AppendFloat`, and
`strconv.FormatComplex` take a format byte among `b`, `e`, `E`, `f`, `g`, `G`,
`x`, and `X`.

The rule is adapted from staticcheck's SA1030.

## Checks

Only a constant argument is checked, because the value of a variable is
unknown. A constant expression counts, and so does a named constant.

The integer parsers take a bit size from 0 to 64. The documentation lists 0,
8, 16, 32, and 64 for the integer types, and every size in the range is valid,
so 1 is left alone.

`strconv.ParseFloat` uses 64 for any bit size other than 32, and
`strconv.ParseComplex` uses 128 for any bit size other than 64. The
documentation lists only that pair, so any other value is outside it.

A format argument is checked as a byte constant, written as a rune or as an
integer.

A call through a function value, as in `f := strconv.ParseInt` followed by
`f(s, 1, 65)`, is left alone, because the analyzer does not follow the value.

Report only. Nothing in the call indicates which valid value was intended.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/strconvrange/cmd/strconvrange
go tool strconvrange ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -strconvrange ./...
```
