# lesiw.io/tools/analyzers/ctxvaluetype

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ctxvaluetype.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ctxvaluetype)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `context.WithValue` call whose value
argument is the nil literal.

```go
func store(ctx context.Context, key any) context.Context {
    return context.WithValue(ctx, key, nil) // context.WithValue is called with the nil literal as its value
}
```

`Context.Value` returns "the value associated with this context for key, or
nil if no value is associated with key". Storing nil adds an entry whose value
is nil. If the parent lacks the key, `Value` still returns nil. If the parent
holds a non-nil value under the key, `Value` returns nil instead of that value.
The rule reports the call regardless of the parent.

Only the nil literal is reported. A typed nil such as `(*T)(nil)` is not a nil
interface, so `Value` returns it. A conversion that yields a nil interface, or
a variable that holds nil, is left alone.

A nil parent, a nil key, or a non-comparable key makes `WithValue` panic before
it stores anything. A nil literal on such a call is reported too, and the report
is at the value argument.

Report only: replacing the call with its parent could change what `Value`
returns where the parent holds a non-nil value.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ctxvaluetype/cmd/ctxvaluetype
go tool ctxvaluetype ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ctxvaluetype ./...
```
