# lesiw.io/tools/analyzers/structconvert

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/structconvert.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/structconvert)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a struct literal that copies every field of
another struct.

```go
type first struct {
    Kind string
}

type second struct {
    Kind string
}

func convert(v first) second {
    return second{Kind: v.Kind} // struct literal copies every field of v; write second(v)
}
```

The Conversions section of the spec says that, ignoring struct tags, a
non-constant value `x` can be converted to type `T` when `x`'s type and `T`
are not type parameters but have identical underlying types
(https://go.dev/ref/spec#Conversions). That conversion is legal even when the
tags differ. This rule requires the tags to be identical. The
literal initializes each field from the field of the same name, so the
conversion and the literal produce the same value.

The rule reports the literal when the two underlying structs are identical,
every destination field appears exactly once, every value is a selector on the
same expression, and the key of a keyed value is the field name its selector
reads. In an unkeyed literal the selectors must follow the field declarations.

A fix replaces the literal with a conversion. A fix is withheld when the
destination type cannot be written at that position, when a name in the rewrite
refers to a different object, when the literal contains a comment, or when the
rewritten line would exceed the line width `linelen` measures. A fix is still
offered when a replaced line is already past the width.

Not reported, and why:

- an elided literal whose element, key, or value type is a pointer: the element
  is implicitly addressed;
- a literal under `&`: a conversion is not addressable, so `&second(v)` does not
  compile;
- a named type or alias declared in another package, directly or through a
  local alias: a
  field added to a dependency makes the conversion illegal while a keyed
  literal still compiles;
- a source and destination of the same type: there is nothing to convert;
- a source whose type is a pointer: the conversion would have to be written on
  the dereference;
- a source expression that can change between reads (a call, a channel
  receive, an address of a literal), read for more than one field: the literal
  does not copy the fields from one value;
- an empty literal: there is no source value to convert;
- a literal that initializes fewer fields than the type has: it does not copy
  every field;
- values selected from more than one expression: no single value is converted;
- a source and destination whose fields differ in name, order, type, tag, or
  whether they are embedded: the structs are not identical.

Adapted from staticcheck's `S1016`, which ignores tags.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/structconvert/cmd/structconvert
go tool structconvert ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -structconvert ./...
go tool lesiw.io/tools/cmd/fix -structconvert ./...
```
