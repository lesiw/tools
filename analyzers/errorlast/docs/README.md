# lesiw.io/tools/analyzers/errorlast

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errorlast.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errorlast)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a function type whose last error result is
followed by another result.

```go
type load func() (error, string) // error is not the last result
```

The error goes last, so a caller writes `value, err := fetch()`.

The Google Go Style Guide says: "By convention, error is the last result
parameter."

The signature of a function declaration, a method declaration or a function
literal is left to `errnotlast`. This rule reports every other function type:
the underlying function type of a type declaration, an interface method, a
field, a variable, a parameter or a result. Both analyzers report a declaration
that names a function type and contains a function literal.

A result is an error result when its type is the predeclared `error`, or a
type that implements `error` under a name ending in `Error`. An alias is
resolved before the suffix test. For a pointer, the suffix test uses the name
of the type it points to, and the pointer is what must implement `error`. A
type whose `Error` method has a pointer receiver is therefore an error only as
a pointer. A final result that implements `error` is accepted, whatever its
name.

A type named differently is left alone: the suffix is how a reader recognizes
an error type. A type parameter is left alone as an earlier result, whatever
its constraint. An unnamed type does not have a name for the suffix test.

A function type whose final result is an error is not reported, even when
another error appears before it. A function type whose error is followed by a
`handled` flag is still reported; the error is still not the last result.

Report only: reordering results changes every call site.

The rule is adapted from revive's error-return, which checks only function
declarations and matches only the predeclared error type. That check is split:
`errnotlast` reports the declarations and literals, and this rule reports the
function types.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errorlast/cmd/errorlast
go tool errorlast ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errorlast ./...
```
