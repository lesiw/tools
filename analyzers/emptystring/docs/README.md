# lesiw.io/tools/analyzers/emptystring

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptystring.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptystring)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a string compared with its own length.

```go
func f(s string) bool {
	return len(s) == 0 // compare the string with "" instead
}
```

len(s) == 0 and s == "" are the same test. The second is the form a reader
expects.

```go
s == ""
```

The idea comes from go-critic's emptyStringTest.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptystring/cmd/emptystring
go tool emptystring ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptystring ./...
go tool lesiw.io/tools/cmd/fix -emptystring ./...
```
