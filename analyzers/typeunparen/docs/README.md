# lesiw.io/tools/analyzers/typeunparen

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/typeunparen.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/typeunparen)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports redundant parentheses in a type expression.

```go
package a

type S struct{}

var x (int) = 1 // redundant parentheses around a type

var y (struct{}) // redundant parentheses around a type

var z (*S) // redundant parentheses around a type

type T (int) // redundant parentheses around a type

var m map[string](int) // redundant parentheses around a type

func g(n int) int {
	return (n + 1)
}
```

Parentheses around a type do not change the type. In front of a call they are
what makes a conversion parse, so the check leaves those alone.

The idea comes from go-critic's typeUnparen.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/typeunparen/cmd/typeunparen
go tool typeunparen ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -typeunparen ./...
go tool lesiw.io/tools/cmd/fix -typeunparen ./...
```
