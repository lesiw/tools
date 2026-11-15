# lesiw.io/tools/analyzers/errorlast

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/errorlast.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/errorlast)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a signature whose error result is not the
last one.

```go
func lookup() (error, int) { // error is not the last result
    return nil, 0
}
```

A caller writes `n, err := lookup()`.

A result is an error result when its type is the predeclared `error`, or a
type that implements `error` under a name ending in `Error`. A pointer is named
after the type it points to, and the pointer is what must implement `error`. A
type whose `Error` method has a pointer receiver is therefore an error only as
a pointer.

The check covers every function type in the package. A type named differently
is left alone: the suffix is how a reader recognizes an error type. A type
parameter is left alone: the caller chooses the type, so the name in the
constraint is not the type's name. An unnamed type has no name for the rule to
read.

The rule asks that nothing follow the last error result. Two errors keep the
order the author gave them, and a trailing `handled` flag does not exempt the
signature: the error is still the value a reader looks for last.

The rule is adapted from revive's error-return, which reads only function
declarations and matches only a type spelled `error`.

Report only: reordering results changes every call site, and an interface
declared elsewhere fixes the order.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/errorlast/cmd/errorlast
go tool errorlast ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -errorlast ./...
```
