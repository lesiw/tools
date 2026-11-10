# lesiw.io/tools/analyzers/unslice

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unslice.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unslice)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a slice expression that takes the whole value.

```go
func f(s []int) []int {
	return s[:] // unnecessary full slice
}
```

Slicing a slice from its start to its end gives the same value back. Slicing an
array converts it to a slice, which is a different value, so the check leaves
an array alone.

```go
s
```

The idea comes from go-critic's unslice.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unslice/cmd/unslice
go tool unslice ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unslice ./...
go tool lesiw.io/tools/cmd/fix -unslice ./...
```
