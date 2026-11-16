# lesiw.io/tools/analyzers/randzero

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/randzero.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/randzero)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `math/rand` or `math/rand/v2`
whose upper bound is one.

```go
func f() int {
    return rand.Intn(1) // the upper bound is 1, so this always returns 0
}
```

`math/rand.Intn` returns a non-negative pseudo-random number in the half-open
interval `[0,n)`. A bound of 1 leaves 0 as the only value, so the call always
returns 0. The upper-bound functions in `math/rand/v2` and the `Rand` methods
in both packages return a value from the same interval.

The functions are the ones that return a number in `[0,n)`: `Int31n`,
`Int63n`, and `Intn` in `math/rand`; `Int32N`, `Int64N`, `IntN`, `N`,
`Uint32N`, `Uint64N`, and `UintN` in `math/rand/v2`; and the methods of the
same name in both packages.

`Perm` and `Shuffle` take an upper bound, but they return a permutation or
rearrange a slice instead of returning a number, so they are not checked.

The bound is reported when it is a constant expression whose value is 1, so a
named constant and an arithmetic expression are both checked. A bound of zero
panics, as does a negative signed bound, so only 1 yields a single value.

A call through a function or method value held in a variable, or through an
interface method, is left alone, because the analyzer reads the callee of a
direct call.

Report only: replacing the call with 0 would stop the generator from advancing,
which changes every later value it produces.

The rule is adapted from staticcheck's SA4030, which matches the literal 1.
This rule also matches a constant expression whose value is 1.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/randzero/cmd/randzero
go tool randzero ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -randzero ./...
```
