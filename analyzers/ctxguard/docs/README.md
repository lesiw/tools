# lesiw.io/tools/analyzers/ctxguard

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ctxguard.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ctxguard)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports redundant `ctx.Err()`
pre-checks.

Go convention passes the context to the first context-aware
operation and lets it observe cancellation itself. A pre-check
duplicates that observation without changing the semantics.

## Checks

### No ctx.Err() pre-checks

```go
func work(ctx context.Context) error {
    if ctx.Err() != nil { // redundant ctx.Err() pre-check: the first context-aware operation observes cancellation
        return ctx.Err()
    }
    return do(ctx)
}
```

The fix removes the pre-check. A comment inside the pre-check
describes the deleted code, so it is removed along with it.

The init-statement form (`if err := ctx.Err(); err != nil`) is the
same pre-check. Loop conditions such as `for ctx.Err() == nil` are
the context-aware operation for their loop and are untouched.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ctxguard/cmd/ctxguard
go tool ctxguard ./...
```
