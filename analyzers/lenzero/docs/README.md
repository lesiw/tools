# lesiw.io/tools/analyzers/lenzero

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/lenzero.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/lenzero)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of len with zero that is always the same.

```go
func f(xs []int) bool {
	return len(xs) < 0 // len is never negative; this is always false
}
```

len returns a number that is never negative. len(x) < 0 is always false, and
len(x) >= 0 is always true. Neither comparison checks anything.

The idea comes from go-critic's sloppyLen.

Fixable: the comparison becomes its answer.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/lenzero/cmd/lenzero
go tool lenzero ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -lenzero ./...
go tool lesiw.io/tools/cmd/fix -lenzero ./...
```
