# lesiw.io/tools/analyzers/slenstr

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/slenstr.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/slenstr)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a string converted to a byte slice only to
take its length.

```go
func size(s string) int {
    return len([]byte(s)) // conversion only to take its length; len counts the bytes directly
}
```

A conversion from a string to a byte slice yields a slice whose elements are
the bytes of the string (https://go.dev/ref/spec#Conversions), and len reports
the length of a string in bytes (https://go.dev/ref/spec#Length_and_capacity).
Both forms count the same bytes, and len(s) does not build the slice.

The rule is inspired by go-critic's stringXbytes, which reports the reverse
length check, `len(string(b))`.

## Checks

`len([]byte(s))` is reported when `s` is a string or has a string underlying
type. The conversion has to produce a slice whose elements are bytes, which
includes a defined byte slice and a slice of a defined byte type.

Four shapes are left alone:

- a conversion of or to a type parameter, whose underlying type is an
  interface (https://go.dev/ref/spec#Underlying_types);
- a conversion whose operand is not a string;
- a conversion whose target is not a byte slice;
- a call to a `len` declared in an enclosing scope, which shadows the builtin.

The fix drops the conversion, leaving `len(s)`. It is withheld when `s` is
constant, since removing the conversion can make the call constant, and when an
edit would delete a comment.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/slenstr/cmd/slenstr
go tool slenstr ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -slenstr ./...
go tool lesiw.io/tools/cmd/fix -slenstr ./...
```
