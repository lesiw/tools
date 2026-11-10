# lesiw.io/tools/analyzers/regexpmust

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/regexpmust.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/regexpmust)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a regexp compiled from a constant with its error dropped.

```go
func must() *regexp.Regexp {
	re, _ := regexp.Compile(`^a+$`) // regexp compiled from a constant; use MustCompile
	return re
}
```

Compiling a constant pattern cannot fail. The error is always nil. MustCompile
does the same and panics if the pattern is wrong.

```go
var re = regexp.MustCompile(`^a+$`)
```

The idea comes from go-critic's regexpMust.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/regexpmust/cmd/regexpmust
go tool regexpmust ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -regexpmust ./...
```
