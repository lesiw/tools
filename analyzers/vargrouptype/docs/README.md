# lesiw.io/tools/analyzers/vargrouptype

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/vargrouptype.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/vargrouptype)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an untyped constant whose default type is
not the one a spec above it declares.

```go
const (
	first  byte = 1
	second      = 2 // second is int; a spec above declares byte
)
```

```go
var (
	limit  byte = 10
	offset      = 20 // offset is int; a spec above declares byte
)
```

`first` is a `byte` and `second` is an untyped integer constant, so its default
type is `int`. A reader of either block expects one type, because the type above
a spec looks like it carries down. The Go spec says it does so only when the spec
omits its expression list too: "Such an empty list is equivalent to the textual
substitution of the first preceding non-empty expression list and its type if
any." A variable declared without a type takes the type of its value instead,
and "if that value is an untyped constant, it is first implicitly converted to
its default type".

A spec is reported when it names an untyped constant whose default type is not
the type above, where that type could hold the value: `2` under a `byte` is
reported, and `2` under an `int` is not. The rule reads `var` blocks as well as
`const` blocks. A typed spec in the middle of a block takes over as the
reference for the specs below it. A spec
that omits its expression list is not reported: the fault belongs to the spec
the list comes from, and naming the type there repairs this one too.

Three repairs are open to the author. Naming the type the value has says what
the spec does: `shiftKind int = 3` is a shift amount, and naming it as an `int`
says so. Naming the block's type gives the value that type, which suits a value
that belongs to the set: `second byte = 2` above is a `byte` like `first`. A
blank line sets the spec apart from the block's set, which suits a value of
another kind written among masks: `shiftKind = 3` under `maskKind kind = 1`.

Exempt, deliberately:

- A value that is not a constant: an index expression, a composite literal, or
  a function literal.
- A conversion or a call. A conversion names its type, and a call is left alone
  even when it yields an untyped constant, as `max(1, 2)` does.
- A value the declared type cannot hold: one of another kind, a fractional
  value under an integer type, a negative value under an unsigned type, or a
  value outside the type's range.
- A declared type that is not a basic type. A struct, map, slice, or func type
  holds no constant at all, and an interface, including the constraint of a type
  parameter, holds one only after a conversion.
- A spec in a new run: a blank line, or a comment on a line of its own, sets it
  apart from the spec above, and the tool compares nothing across one.
- The blank identifier, whose type nothing reads.

Report only: each repair changes what the value means, and the tool cannot tell
which one the author meant.

The rule is adapted from staticcheck's SA9004, which reads a const block whose
first spec is typed, reads a literal value only, and adds the type to the whole
group.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/vargrouptype/cmd/vargrouptype
go tool vargrouptype ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -vargrouptype ./...
```
