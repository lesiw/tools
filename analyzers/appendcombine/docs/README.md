# lesiw.io/tools/analyzers/appendcombine

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/appendcombine.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/appendcombine)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports consecutive appends to the same slice.

```go
func f() []int {
	var xs []int
	xs = append(xs, 1) // 2 appends to one slice
	xs = append(xs, 2)
	return xs
}
```

Each append may grow the slice and copy it. One append with every value grows
it once.

A spread append cannot join another, because append takes a single `...`.

```go
xs = append(xs, a, b, c)
```

The idea comes from go-critic's appendCombine.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/appendcombine/cmd/appendcombine
go tool appendcombine ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -appendcombine ./...
```
