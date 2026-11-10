# lesiw.io/tools/analyzers/duplarg

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/duplarg.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/duplarg)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a call that passes one value where it expects two.

```go
func f(s string) bool {
	return strings.Contains(s, s) // both arguments are the same value
}
```

f(x, x) gives both parameters one value. strings.Contains(x, x) is always true,
math.Max(x, x) is x, and copy(x, x) copies a slice onto itself. The call looks
like two values were meant.

The idea comes from go-critic's dupArg.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/duplarg/cmd/duplarg
go tool duplarg ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -duplarg ./...
```
