# lesiw.io/tools/analyzers/selfassert

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/selfassert.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/selfassert)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a type assertion from a type to itself.

```go
func f(r any) any {
	return r.(any) // type assertion from a type to itself
}
```

An assertion from a type to itself always succeeds and returns the value it was
given. It does nothing.

The idea comes from go-critic's sloppyTypeAssert.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/selfassert/cmd/selfassert
go tool selfassert ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -selfassert ./...
```
