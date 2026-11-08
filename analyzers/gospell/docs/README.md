# lesiw.io/tools/analyzers/gospell

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/gospell.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/gospell)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a word spelled the way Go does not spell
it. Go writes American English: `context.Canceled` has one l, and the standard
library has no "cancelled" anywhere.

The rule covers comments and string literals, which are prose. An identifier is
left alone, since it may be a name this package does not own.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/gospell/cmd/gospell
go tool gospell ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -gospell ./...
go tool lesiw.io/tools/cmd/fix -gospell ./...
```
