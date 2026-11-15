# lesiw.io/tools/analyzers/ctxkey

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ctxkey.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ctxkey)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `context.WithValue` key of a
built-in type.

```go
func f(ctx context.Context, v any) context.Context {
	return context.WithValue(ctx, "some-key", v) // context key has the built-in type string; define a key type
}
```

The documentation for `context.WithValue` says the key "should not be of type
string or any other built-in type to avoid collisions between packages using
context", and that users "should define their own types for keys". A key whose
type is built in, such as `string`, `int`, or `bool`, is the same type in
every package, so two packages that pick the same value collide.

The rule stands down once the key has a defined type, in this package or in
another: two packages may declare a type of the same name, and the two types
differ. An alias is the same type under another name, so a key of an alias to
a string still collides and is still reported. The rule reads the type the
argument has at the call; a type that is abstract there, an interface or a
type parameter, is left alone, because the type it stands for is not visible.
Neither is a `nil` key: `WithValue` panics on one, so no value is ever stored
under it.

The suite's other context analyzers report on the context: `ctxfirst` a
parameter that is not first, `ctxfield` one in a struct field, `ctxname` one
named something else, and `ctxguard` a redundant `ctx.Err()` pre-check. This
one reports the key the context carries.

The rule is adapted from staticcheck's `SA1029`. Upstream also reports a key of
non-comparable type, which panics in `WithValue`, and a key of empty anonymous
struct type, which is not a built-in type.

Report only: the edit would have to invent a key type and a name for it, and
that name becomes part of the package's API.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ctxkey/cmd/ctxkey
go tool ctxkey ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ctxkey ./...
```
