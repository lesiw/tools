# lesiw.io/tools/analyzers/ctxfirst

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ctxfirst.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ctxfirst)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a context parameter that is not the first one.

```go
func second(n int, ctx context.Context) { // context.Context is not the first parameter
}
```

A context has the deadline and the cancellation for one request. Every function
in the chain takes it as a parameter. A reader finds it in the same place in
every signature when it comes first.

The idea comes from revive's context-as-argument.

Report only: moving a parameter changes every call site.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ctxfirst/cmd/ctxfirst
go tool ctxfirst ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ctxfirst ./...
```
