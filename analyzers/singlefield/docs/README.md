# lesiw.io/tools/analyzers/singlefield

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/singlefield.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/singlefield)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports single-field structs
replaceable by their underlying type.

A type carrying a single payload does not need the struct wrapper:
the value at the call site is the payload, no field access needed.

## Checks

### One field, no struct

```go
type ID struct{ s string } // single-field struct can be replaced by its underlying type: ID string
```

The fold — the type and every use, `X{f: v}` to `X(v)`, `x.f` to
`T(x)` — is the author's work. For an exported type it changes the
API of every importer, so it is a release decision.

Exempt, deliberately:

- A struct whose one field is an interface: the folded type would
  itself be an interface, not a concrete type.
- An embedded field, or a field with a struct tag: promotion and
  serialization shape are load-bearing.
- A pointer field: a named pointer type cannot be a method
  receiver, so the folded type could never carry behavior.

Type aliases and generic types are not reported.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/singlefield/cmd/singlefield
go tool singlefield ./...
```
