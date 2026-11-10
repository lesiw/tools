# lesiw.io/tools/analyzers/newderef

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/newderef.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/newderef)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a dereference of a fresh allocation.

```go
func f() int {
	return *new(int) // dereferenced allocation
}
```

new(T) allocates a zero T and returns the pointer. Dereferencing it gives back
the zero value. Writing the zero value directly is shorter.

The idea comes from go-critic's newDeref.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/newderef/cmd/newderef
go tool newderef ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -newderef ./...
```
