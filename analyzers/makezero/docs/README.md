# lesiw.io/tools/analyzers/makezero

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/makezero.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/makezero)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a slice made with a non-zero length that is appended to.

```go
func f() []int {
	s := make([]int, 10) // slice made with a length, then appended to
	return append(s, 1)
}
```

make([]T, n) allocates n elements and sets the length to n. An append after it
writes past those zeros, and the zeros stay in the result. make([]T, 0, n)
allocates the same capacity with a length of zero.

```go
make([]T, 0, n)
```

The idea comes from the makezero linter.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/makezero/cmd/makezero
go tool makezero ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -makezero ./...
go tool lesiw.io/tools/cmd/fix -makezero ./...
```
