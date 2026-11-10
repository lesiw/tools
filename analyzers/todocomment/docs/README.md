# lesiw.io/tools/analyzers/todocomment

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/todocomment.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/todocomment)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a TODO, FIXME or BUG doc comment with no detail.

```go
func bare() {} // note with no detail

// FIXME: fix this
func withDetail() {}

// TODO(team): finish
func withOwner() {}

// A comment that mentions a TODO in passing is fine.
func mention() {}

// BUG
func bug() {} // note with no detail

// BUG()
func emptyParens() {} // note with no detail
```

A note with no detail gives the next reader nothing to act on. A search for the
marker returns every note, finished or not.

```go
// TODO: accept a nil receiver.
func f() {}
```

The idea comes from go-critic's todoCommentWithoutDetail.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/todocomment/cmd/todocomment
go tool todocomment ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -todocomment ./...
```
