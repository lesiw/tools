# lesiw.io/tools/analyzers/exposedsyncmutex

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/exposedsyncmutex.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/exposedsyncmutex)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `sync.Mutex` or `sync.RWMutex` embedded in
a type an importer can reach.

```go
type Cache struct {
	sync.Mutex // avoid embedding sync.Mutex; consider a named field
}
```

An embedded field is named after its type, so the embed gives the struct a field
named `Mutex`. For an embedded `sync.Mutex` the pointer to the type gains `Lock`
and `Unlock`; for an embedded `*sync.Mutex` the type itself gains them, so a
value of the type satisfies `sync.Locker`. A caller can unlock the mutex a
method holds, so the invariant the lock exists for no longer holds, and a caller
that holds the lock deadlocks the next method that takes it.

The report is on the embed, and the message names the type as the package
declares it, by package name rather than import path.

The rule starts from the type an exported type declaration declares, the type of
an exported variable, and the result types of an exported function.

Pointers are followed. From those types the rule follows fields the package
embeds or exports, the results of the methods an importer can call, the exported
methods of an interface, slice and array elements, map keys and elements, channel
elements, and the result types of a function type.

The rule reports an embedded field whose type carries sync's `Lock` and
`Unlock`: the lock itself, a struct or interface another package declares that
promotes or declares them, or a defined type over such a struct. A type that
declares `Lock` and `Unlock` over the embed is reported as well: the embed is
still the line to change.

The rule leaves alone:

- A lock in a named field, `mu sync.Mutex` or `Mu sync.Mutex`: a named field
  does not add `Lock` and `Unlock` to the type's method set.
- A lock in the result of a method no importer can call: one with an unexported
  name, or on a type no exported name reaches.
- An embed written in another package's type: the rule reads the field lists
  this package writes, and the run over that package holds the embed.
- A `main` package, which nothing can import, and a test file, which no importer
  compiles.

The rule is adapted from go-critic's `exposedSyncMutex`.

Report only: the repair is a named field with `Lock` and `Unlock` declared over
it, so the mutex's methods stay out of the type's method set, and the analyzer
cannot choose the name. Every selector of the lock changes with the repair.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/exposedsyncmutex/cmd/exposedsyncmutex
go tool exposedsyncmutex ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -exposedsyncmutex ./...
```
