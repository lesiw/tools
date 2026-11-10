# lesiw.io/tools/analyzers/indexalloc

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/indexalloc.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/indexalloc)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a byte slice searched as a string.

```go
func f(b []byte, s string) int {
	return strings.Index(string(b), s) // string conversion only to search; use bytes.Index
}
```

Converting a byte slice to a string to search it allocates a copy of the bytes.
The bytes package searches the slice in place.

```go
bytes.IndexByte(b, c)
```

The idea comes from go-critic's indexAlloc.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/indexalloc/cmd/indexalloc
go tool indexalloc ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -indexalloc ./...
```
