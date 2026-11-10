# lesiw.io/tools/analyzers/yoda

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/yoda.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/yoda)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison with the constant on the left.

```go
func f(err error) bool {
	return nil == err // constant on the left
}
```

Go code reads left to right as a sentence about the variable: err is nil. The
constant-first form comes from C, where it guarded against a mistyped
assignment. That mistake does not compile in Go.

```go
err == nil
```

The idea comes from go-critic's yodaStyleExpr.

Fixable: the operands are swapped.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/yoda/cmd/yoda
go tool yoda ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -yoda ./...
go tool lesiw.io/tools/cmd/fix -yoda ./...
```
