# lesiw.io/tools/analyzers/tidytypes

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/tidytypes.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/tidytypes)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports types spelled out where the compiler can
infer them.

`var day int = 0` and `var day = 0` say the same thing; the second is the one
the compiler would have written. A redundant type is a claim about the code
that the code does not need, and it goes stale the moment the value changes
kind.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/tidytypes/cmd/tidytypes
go tool tidytypes ./...
```
