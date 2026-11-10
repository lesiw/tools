# lesiw.io/tools/analyzers/ifelsechain

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ifelsechain.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ifelsechain)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if-else chain that reads as a switch.

```go
func three(n int) int {
	if n == 0 { // if-else chain of 4 branches; use a switch
		return 0
	} else if n == 1 {
		return 1
	} else if n == 2 {
		return 2
	} else if n == 3 {
		return 3
	}
	return -1
}
```

A chain of four or more branches on one value is a switch written as ifs. A
switch puts the value at the top, where a reader looks for it. A shorter chain
is a choice between two things, and an if is the right form for that.

```go
switch n {
case 0:
	return 0
}
```

The idea comes from go-critic's ifElseChain.

Report only: the rewrite moves every branch.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ifelsechain/cmd/ifelsechain
go tool ifelsechain ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ifelsechain ./...
```
