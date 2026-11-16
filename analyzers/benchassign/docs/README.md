# lesiw.io/tools/analyzers/benchassign

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/benchassign.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/benchassign)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an assignment to the `N` field of a
`testing.B`.

```go
b.N = 100
```

The [`testing` package](https://pkg.go.dev/testing) documents
[`B`](https://pkg.go.dev/testing#B) as "a type passed to `Benchmark` functions
to manage benchmark timing and control the number of iterations". Its
benchmark documentation says the benchmark function is called multiple times
with `b.N` adjusted until the run lasts long enough to be timed reliably. `N`
has no doc comment; the benchmark section documents it. The testing package
reports the measured duration against `N`, so an assignment to `N` makes the
reported `ns/op` describe a different iteration count.

The write may be a plain assignment, a compound assignment, an increment or
decrement, or a range clause. A read of `b.N` is not reported, and neither is a
field named `N` on another type. The rule reports a write whose receiver is a
`*testing.B`, including a type alias or a defined pointer type, and a write
promoted through embedded pointers to one. A struct that embeds `testing.B` by
value holds a copy and is left alone. A selector on a value of type `testing.B`
is left alone, as is one on a defined type whose underlying type is
`testing.B`. Taking the address of such a value gives a pointer, and a write
through it is reported. Writes in the `testing` package itself are not
reported.

Report only: only the author knows what count a benchmark should run, and
removing the assignment changes what the benchmark measures.

The rule is inspired by staticcheck's `SA3001`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/benchassign/cmd/benchassign
go tool benchassign ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -benchassign ./...
```
