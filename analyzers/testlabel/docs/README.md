# lesiw.io/tools/analyzers/testlabel

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/testlabel.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/testlabel)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a subtest name is string literal.

A subtest names a case in a family: it exists so a table's rows can be told
apart in a failure message, or so a loop's iterations can. A literal name
names one thing, so the subtest reads as a sentence about the code rather than
a case in it, and a failure sends a reader looking for a subtest that has
nothing to tell them.

A name read from a table row, built at run time, or held in a variable is left
alone. A constant that folds to a literal is not.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/testlabel/cmd/testlabel
go tool testlabel ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -testlabel ./...
go tool lesiw.io/tools/cmd/fix -testlabel ./...
```
