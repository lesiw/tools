# lesiw.io/tools/analyzers/unusedctxkey

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unusedctxkey.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unusedctxkey)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a context key constant with no use outside
a `Context.Value` lookup.

```go
type key int

const (
    usedKey key = iota
    deadKey // context key deadKey is never used with context.WithValue
)

func store(ctx context.Context, v any) context.Context {
    return context.WithValue(ctx, usedKey, v)
}
```

The `Context.Value` doc says that a function which stores a value in a Context
"typically allocate[s] a key in a global variable then use[s] that key as the
argument to context.WithValue and Context.Value". A constant with no reference
outside a `Value` key has no store under its name in the package: the call that
would use it is missing.

The analyzer reads the key type from the key argument of a `context.WithValue`
or `Context.Value` call in the package. A constant of one of those types is
reported when its only references are `Context.Value` keys. Any other
reference, such as a `WithValue` key, a store or a comparison, leaves the
constant alone. A constant anywhere inside a `Value` key expression counts as
a `Value` key reference. A constant declared in a function is checked the same
way.

A `Value` method stored in a variable is not followed, so its key argument
counts as an ordinary use. Context compares keys by value, so a reported
constant can read a value that another constant of equal value stores. The
report concerns the name.

Not reported, and why:

- a constant of a type that appears in no context call in the package: the
  analyzer cannot tell that the type is a key type;
- a constant of a built-in type: every string or int shares that type, so it
  does not show that the constant is a key;
- a package-scope exported constant in an importable package: another package
  can pass it to `WithValue`;
- a constant the package reads outside a `Value` key, including a `WithValue`
  key, a store or a comparison;
- the blank identifier.

A constant used only as the key of a `Value` call is reported: no `WithValue`
call stores a value under that name.

Report only: the edit could delete the constant or add the store that gives it
a value, and the analyzer cannot choose between them.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unusedctxkey/cmd/unusedctxkey
go tool unusedctxkey ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unusedctxkey ./...
```
