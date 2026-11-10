# lesiw.io/tools/analyzers/strconcat

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/strconcat.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/strconcat)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a two- or three-element Join with a literal separator.

```go
func f(a, b string) string {
	return strings.Join([]string{a, b}, "-") // Join of 2 parts
}
```

strings.Join allocates a slice for its arguments and then walks it. For two or
three parts, concatenation allocates once and needs no slice literal.

```go
a + "-" + b
```

The idea comes from go-critic's stringConcatSimplify.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/strconcat/cmd/strconcat
go tool strconcat ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -strconcat ./...
go tool lesiw.io/tools/cmd/fix -strconcat ./...
```
