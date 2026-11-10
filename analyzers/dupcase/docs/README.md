# lesiw.io/tools/analyzers/dupcase

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupcase.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupcase)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a case that names an expression an earlier case named.

```go
func f(xs []int) int {
	switch xs[0] {
	case xs[1], xs[2], xs[1]: // case repeats an earlier expression
		return 1
	case xs[3]:
		return 2
	}
	return 0
}
```

The second case can never match. Either the author mistyped one of the two, or
the two were meant to be one case. The compiler catches a repeated constant, so
the check reports only expressions it cannot fold.

The idea comes from go-critic's dupCase.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupcase/cmd/dupcase
go tool dupcase ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupcase ./...
```
