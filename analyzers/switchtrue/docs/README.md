# lesiw.io/tools/analyzers/switchtrue

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/switchtrue.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/switchtrue)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a switch whose tag is the literal true.

```go
func f(n int) int {
	switch true { // switch on true; drop the tag
	case n > 0:
		return 1
	default:
		return 0
	}
}
```

A switch on true compares every case with true. The tagless form does the same
with one word fewer.

```go
switch {
case n > 0:
	return 1
}
```

The idea comes from go-critic's switchTrue.

Fixable: the tag is removed, and an init statement is kept.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/switchtrue/cmd/switchtrue
go tool switchtrue ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -switchtrue ./...
go tool lesiw.io/tools/cmd/fix -switchtrue ./...
```
