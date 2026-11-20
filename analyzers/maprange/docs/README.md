# lesiw.io/tools/analyzers/maprange

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/maprange.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/maprange)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call to `maps.Keys` or `maps.Values` in
a range.

```go
func f(m map[string]int) {
    for name := range maps.Keys(m) { // unnecessary and inefficient call of maps.Keys
        use(name)
    }
}
```

The `maps` package documents [`Keys`](https://pkg.go.dev/maps#Keys) as an
iterator over the keys in the map and
[`Values`](https://pkg.go.dev/maps#Values) as an iterator over its values. A
range over a map yields the key and the value, so the loop above becomes
`for name := range m`.

`maps.Values` has no such shorthand, so the value moves into the second
position and the key becomes blank:

```go
func f(m map[string]int) {
    for count := range maps.Values(m) { // unnecessary and inefficient call of maps.Values
        use(count)
    }
}
```

The loop becomes `for _, count := range m`.

## golang.org/x/exp/maps

`golang.org/x/exp/maps` returns slices where the standard library returns
iterators, so the single variable of such a loop is an index into the slice,
not a key of the map:

```go
func f(m map[string]int) {
    for i := range maps.Keys(m) { // unnecessary and inefficient call of golang.org/x/exp/maps.Keys
        use(i)
    }
}
```

It becomes `for i := range len(m)`, which reads the length once instead of
building a copy of the keys or values.

That rewrite needs Go 1.22. In a file checked against an earlier version the
counting loop is reported as `likely incorrect use of ... (returns a slice)`,
without a fix, because the index of a slice of keys or values is rarely what
the author meant. At Go 1.22 and later the same loop is reported as an
unnecessary call and rewritten.

A loop that binds both, `for i, key := range maps.Keys(m)`, is not reported: the
index of a slice of keys is not a key of the map, so no edit keeps the loop's
meaning.

## When the fix is withheld

The fix is withheld for three reasons.

It would change what the loop does. A loop over the `golang.org/x/exp/maps`
copy that binds the element, or binds nothing, is reported without a fix,
because ranging the map instead iterates a different set of entries: a body
that adds an entry would then see it. A local `len` would take the call the fix
writes, and is reported the same way.

It would delete a comment between the two variables of the range clause, or
inside the call.

It would leave the file uncompilable. A call named through a dot import is
reported without a fix: removing the call can strand the import, and `goimports`
keeps an unused dot import.

Vendored from gopls' `maprange` (`gopls/internal/analysis/maprange`,
`golang.org/x/tools/gopls` v0.24.0), which is internal and cannot be imported.
The rule and its messages are upstream's (BSD-3-Clause, Copyright 2025 The Go
Authors); the code is adapted to this repository's helpers. Upstream takes the
call out of `for i, _ := range maps.Keys(m)`, which changes what `i` is
assigned; this ranges over `len(m)` instead. A file with no version of its own
is checked with no version enforced, where upstream reads it as older than Go
1.22.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/maprange/cmd/maprange
go tool maprange ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -maprange ./...
go tool lesiw.io/tools/cmd/fix -maprange ./...
```
