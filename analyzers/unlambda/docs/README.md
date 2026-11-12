# lesiw.io/tools/analyzers/unlambda

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unlambda.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unlambda)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a function literal that only forwards its arguments.

```go
func g() {
	use(func(s string) []string { return strings.Fields(s) }) // literal only forwards its arguments
	use(strings.Fields)
}
```

The literal passes its arguments through in order and returns the result.
Naming the function does the same with less code. A method value closes over
its receiver, so the check skips one.

A literal whose type is wider than the function's is left alone: returning an
interface where the function returns a concrete type cannot be shortened to the
name.

The idea comes from go-critic's unlambda.

Fixable: the literal becomes the function.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unlambda/cmd/unlambda
go tool unlambda ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unlambda ./...
go tool lesiw.io/tools/cmd/fix -unlambda ./...
```
