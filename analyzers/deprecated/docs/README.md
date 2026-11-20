# lesiw.io/tools/analyzers/deprecated

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deprecated.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deprecated)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a use of a deprecated identifier or
package.

```go
legacy.Old() // legacy.Old is deprecated: Use Current instead.
```

Vendored from gopls' `deprecated` (`gopls/internal/analysis/deprecated`,
`golang.org/x/tools/gopls` v0.24.0), which is internal and cannot be imported.
The rule is upstream's; the code is adapted to this repository's helpers.

## Where a note is read

A note is a paragraph that begins with `Deprecated: `, as
[the Go wiki](https://go.dev/wiki/Deprecated) defines it. It is read from:

- the doc comment of a function, method, type, constant or variable;
- the doc comment of a type, const or var group, which marks every name in the
  group. A spec that carries its own note overrides the group's;
- the doc comment or the line comment of a struct field and an interface
  method;
- the package doc, in whichever file holds it.

A line comment may also hold the note after other prose. A doc comment wins
over a line comment where both carry one. A note written across lines is joined
into one line.

## Checks

### A deprecated object

A selector that names a deprecated function, type, constant or variable is
reported where it is written:

```go
_ = legacy.Limit // legacy.Limit is deprecated: Use Cap instead.
```

A deprecated struct field and interface method are reported the same way:

```go
_ = item.DocName     // item.DocName is deprecated: Use Name instead.
handler.OldAccept(1) // handler.OldAccept is deprecated: Use Accept instead.
```

### A deprecated package

An import of a package whose doc comment carries a note is reported at the
import path:

```go
import "stale" // "stale" is deprecated: Use the fresh package instead.
```

A use of an object from that package is reported only when the object carries a
note of its own.

## Exemptions

- A package may use its own deprecated objects, so a deprecated method of a
  local type is not reported where the package calls it.
- A test package may use the package it tests: the external tests of `foo`, and
  the main package that `go test` generates, which may use both.

## Not reported

A dot import puts no selector in the source, so a use of a deprecated object
from a dot-imported package is not reported. A deprecated package is still
reported at the import.

A field named as a composite literal key is not a selector either, so a
deprecated field written that way is not reported.

A concrete method that shares its name with a deprecated interface method is a
different declaration, so calling it is not reported.

Report only: a note is prose, and the tool cannot turn prose into an edit.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deprecated/cmd/deprecated
go tool deprecated ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deprecated ./...
```
