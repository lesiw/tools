# lesiw.io/tools/analyzers/makecap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/makecap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/makecap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a capacity argument equal to `make`'s
default.

```go
func sized(n int) []int {
    return make([]int, n, n) // capacity can be omitted; it equals the length
}
```

```go
func events() chan int {
    return make(chan int, 0) // capacity can be omitted; zero is unbuffered
}
```

`make`'s documentation says "The capacity of the slice is equal to its
length", so a slice's third argument is redundant when it has the same value as
the second. It says a channel "is unbuffered" when its size is zero or omitted,
so a constant zero capacity is redundant too.

A slice's length and capacity are compared as values:

- the same identifiers, fields, literals, indexes, and dereferences;
- constants that are equal, such as `2+1` and `3`;
- the same expression whose value cannot change between the two evaluations,
  such as `len(xs)`.

A call that can return a different value is left alone, and so is a length that
reads a channel, which another goroutine can change: `make([]int, len(ch),
len(ch))` does not always have equal length and capacity.

A slice with a capacity larger than its length is left alone:
`make([]int, 0, n)` gives a length of zero and capacity `n`, which leaves spare
capacity for append. A channel with a non-constant size is left alone, since
its capacity is not known here to be zero. A map's size argument is a size
hint, so it is left alone. A make call whose type argument is a type parameter
is left alone.

The fix deletes the capacity argument when it repeats the length, or when every
local variable and import it uses is used outside the reported capacities. A
local variable read only there, or an import used only there, would become
unused, and Go rejects both. When two reported capacities are the only uses of
a local variable or import, neither fix is offered. The fix is also withheld
when a comment is in the text the fix deletes. The diagnostic is still reported
when the fix is withheld.

The rule is adapted from staticcheck's `S1019`, which simplifies a make call by
omitting redundant arguments.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/makecap/cmd/makecap
go tool makecap ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -makecap ./...
go tool lesiw.io/tools/cmd/fix -makecap ./...
```
