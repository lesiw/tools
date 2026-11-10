# lesiw.io/tools/analyzers/valswap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/valswap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/valswap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a swap written through a temporary.

```go
func f(x, y int) (int, int) {
	tmp := x // swap through tmp; write x, y = y, x instead
	x = y
	y = tmp
	return x, y
}
```

Go evaluates every right-hand side before it assigns any of them. x, y = y, x
therefore swaps two values with no temporary. The temporary came from languages
that assign one variable at a time.

```go
a, b = b, a
```

The idea comes from go-critic's valSwap.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/valswap/cmd/valswap
go tool valswap ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -valswap ./...
```
