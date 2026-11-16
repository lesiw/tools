# lesiw.io/tools/analyzers/fieldcopy

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/fieldcopy.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/fieldcopy)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a struct literal that copies every field of
a same-shaped struct whose tags differ.

```go
type first struct {
    Kind string `json:"kind"`
}

type second struct {
    Kind string `json:"type"`
}

func f(v first) second {
    return second{Kind: v.Kind} // struct literal copies every field of v; write second(v)
}
```

The Conversions section of the spec ignores struct tags when it compares two
struct types, so a value of one type converts to the other even when their tags
differ (https://go.dev/ref/spec#Conversions). The conversion produces the
destination type, tags included, so it and the literal have the same value.

`structconvert` reports the same literal when the tags are identical. This rule
reports the literal when the tags differ.

The rule reports the literal when the two underlying structs are identical
ignoring tags, every destination field appears exactly once, every value is a
selector on the same expression, and the key of a keyed value is the field name
its selector reads. In an unkeyed literal the selectors must follow the field
declarations.

A fix replaces the literal with a conversion. A fix is withheld when the
destination type cannot be written at that position, when a local declaration
hides the type name, when the literal contains a comment, or when the rewritten
line would exceed the line width `linelen` measures. A fix is still offered
when a replaced line is already past the width.

Not reported, and why:

- a source and destination whose tags are identical: `structconvert` reports
  that case;
- a source and destination of the same type: there is nothing to convert;
- an elided literal whose element, key, or value type is a pointer: the element
  is implicitly addressed, and a conversion is not addressable;
- a literal under `&`: a conversion is not addressable;
- a named type or alias declared in another package: a field added to a
  dependency makes the conversion illegal while a keyed literal still compiles;
- a source expression that can change between reads (a call or a channel
  receive), read for more than one field: the literal does not copy the fields
  from one value;
- an empty literal or a literal that initializes fewer fields than the type
  has: it does not copy every field;
- values selected from more than one expression: no single value is converted;
- a source and destination whose fields differ in name, order, type, or
  whether they are embedded: the structs are not identical ignoring tags.

Adapted from staticcheck's `S1016`, which ignores tags.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/fieldcopy/cmd/fieldcopy
go tool fieldcopy ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -fieldcopy ./...
go tool lesiw.io/tools/cmd/fix -fieldcopy ./...
```
