# lesiw.io/tools/analyzers/simplifycompositelit

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/simplifycompositelit.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/simplifycompositelit)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an element or map key that repeats the type
its composite literal names.

```go
var _ = []pair{
    pair{1, 2}, // redundant type in slice literal
}
```

## The spec

The spec lets an element or key that is itself a composite literal elide its
type, when that type is identical to the element or key type of the literal
around it
([spec](https://go.dev/ref/spec#Composite_literals)). An element or key of type
`*T` may elide the `&` of an address too.

This is one of the simplifications
[`gofmt -s`](https://pkg.go.dev/cmd/gofmt#hdr-The_simplify_command) applies.
`vet` and `fix` do not run `gofmt -s`, so a tree that is already formatted with
it has nothing for this rule to report.

## Checks

An element or key is compared by type, so an alias matches on either side:

```go
type aliasPair = pair

var _ = []pair{
    aliasPair{}, // redundant type in slice literal
}

var _ = []aliasPair{
    pair{}, // redundant type in slice literal
}
```

A map is compared on its key type and on its value type, so both halves of an
entry are reported:

```go
var _ = map[pair]code{
    pair{1, 2}: code{3, 4}, // redundant type in map literal
}
```

The outer literal's own type is read from its syntax, so it has to be written as
an array, slice, or map type. A literal written through a name is not looked
into, as in `pairList{ pair{} }` under `type pairList = []pair`.

## Not reported

- an element whose explicit type is not identical to the element type, as in
  `[]interface{}{pair{}}`, because that element type is the interface;
- an element in parentheses, which the check does not look through, as in
  `[][]int{ ([]int{}) }`.

## Fixes

The fix deletes the type, and the `&` with it when the value or key has an
address. Those are two edits, so a comment between the `&` and the type
survives:

```go
var _ = []*pair{
    & /* keep */ pair{},
}
```

becomes

```go
var _ = []*pair{
    /* keep */ {},
}
```

A comment inside the type belongs to the text the edit deletes, so the fix is
withheld:

```go
var _ = [][]int{
    [] /* keep */ int{},
}
```

The diagnostic is reported either way. `gofmt -s` keeps the comment, and the
edit would drop it.

## Provenance

Vendored from gopls' `simplifycompositelit`
(`gopls/internal/analysis/simplifycompositelit`, `golang.org/x/tools/gopls`
v0.24.0), which is internal and cannot be imported. The rule and its diagnostic
messages are upstream's (BSD-3-Clause, Copyright 2020 The Go Authors); the code
is adapted to this repository's helpers. Upstream offers the fix
unconditionally; this repository withholds it where the edit would delete a
comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/simplifycompositelit/cmd/simplifycompositelit
go tool simplifycompositelit ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -simplifycompositelit ./...
go tool lesiw.io/tools/cmd/fix -simplifycompositelit ./...
```
