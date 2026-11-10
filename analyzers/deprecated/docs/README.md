# lesiw.io/tools/analyzers/deprecated

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deprecated.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deprecated)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deprecation that godoc will not read.

```go
func Older() {} // deprecation without the "Deprecated:" prefix

// deprecated, use New.
func Oldest() {} // deprecation without the "Deprecated:" prefix

// New is the replacement.
func New() {}

// This one is not deprecated at all.
func Current() {}

// deprecatedFoo is not a deprecation note; the word is part of a name.
func deprecatedFoo() {}
```

godoc and every editor that renders it only recognize a paragraph that begins
with Deprecated: . A comment that opens with the word instead is prose. A
caller gets no warning.

```go
// Deprecated: use New instead.
func Old() {}
```

The idea comes from go-critic's deprecatedComment.

Report only: the note has to say what replaces it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deprecated/cmd/deprecated
go tool deprecated ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deprecated ./...
```
