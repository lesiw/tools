# lesiw.io/tools/analyzers/assignop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/assignop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/assignop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an assignment that writes a variable in terms of itself.

```go
func f(x, y int) int {
	x = x + 1 // assignment writes x in terms of itself
	x = x * y // assignment writes x in terms of itself
	x += 1
	return x
}
```

The assignment names the variable twice and the operation once. The compound
form names each once.

```go
x += y
```

The idea comes from go-critic's assignOp.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/assignop/cmd/assignop
go tool assignop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -assignop ./...
go tool lesiw.io/tools/cmd/fix -assignop ./...
```
