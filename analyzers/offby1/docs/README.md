# lesiw.io/tools/analyzers/offby1

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/offby1.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/offby1)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an index that is always out of range.

```go
func f(xs []int) int {
	return xs[len(xs)] // index out of range
}
```

len(x) is the number of elements, so the last one is at len(x)-1. Indexing at
len(x) panics on every call. The compiler accepts it.

The idea comes from go-critic's offBy1.

Report only: which index was meant is the author's to say.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/offby1/cmd/offby1
go tool offby1 ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -offby1 ./...
```
