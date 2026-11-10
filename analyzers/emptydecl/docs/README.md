# lesiw.io/tools/analyzers/emptydecl

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptydecl.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptydecl)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an empty declaration block.

```go
package a

var () // empty var block

const () // empty const block

type () // empty type block

var x = 1

const y = 2

type S struct{}
```

var (), const () and type () declare nothing. The block was emptied and left
behind.

The idea comes from go-critic's emptyDecl.

Fixable: the block and its line are removed.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptydecl/cmd/emptydecl
go tool emptydecl ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptydecl ./...
go tool lesiw.io/tools/cmd/fix -emptydecl ./...
```
