# lesiw.io/tools/analyzers/increment

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/increment.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/increment)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an increment or decrement written as a
compound assignment.

```go
func f(i int) int {
    i += 1 // += 1 can be replaced by ++
    i -= 1 // -= 1 can be replaced by --
    return i
}
```

The spec gives `x++` and `x += 1` the same meaning. Go code mostly writes `x++`,
and the rule reports the exceptions.

Only an operand whose underlying type is an integer is reported. The restriction
is deliberate: `f++` on a float is legal and means the same as `f += 1`, but Go
code rarely writes it. A type parameter is left alone even under an integer
constraint, because the operand's underlying type is then an interface. The
literal must be written `1`, so `i += 2`, `i += one`, `i += 1.0`, and `i += 0x1`
are left alone.

The rule is adapted from revive's increment-decrement, which reports every
operand type.

Fixable: `i += 1` becomes `i++`, and `*p += 1` becomes `(*p)++`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/increment/cmd/increment
go tool increment ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -increment ./...
go tool lesiw.io/tools/cmd/fix -increment ./...
```
