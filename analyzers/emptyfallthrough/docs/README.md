# lesiw.io/tools/analyzers/emptyfallthrough

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptyfallthrough.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptyfallthrough)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a case whose whole body is a fallthrough.

```go
func f(n int) int {
	switch n {
	case 0:
		fallthrough // case only falls through
	case 1:
		return 1
	}
	return 0
}
```

The case contains one statement, and that statement passes control to the next
case. Listing both values in one case does the same without the jump.

A fallthrough into default is a different shape: the case is redundant
outright, because an unmatched value reaches default anyway.

```go
case 0, 1:
```

The idea comes from go-critic's emptyFallthrough.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptyfallthrough/cmd/emptyfallthrough
go tool emptyfallthrough ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptyfallthrough ./...
go tool lesiw.io/tools/cmd/fix -emptyfallthrough ./...
```
