# lesiw.io/tools/analyzers/deferunlambda

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferunlambda.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferunlambda)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred literal that only calls a function.

```go
func f() {
	defer func() { close1() }() // deferred literal only calls a function
}
```

The deferred literal calls one function and passes it nothing. The defer
statement evaluates its arguments where it stands. The wrapper adds a closure
and nothing else.

```go
defer f()
```

The idea comes from go-critic's deferUnlambda.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferunlambda/cmd/deferunlambda
go tool deferunlambda ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferunlambda ./...
go tool lesiw.io/tools/cmd/fix -deferunlambda ./...
```
