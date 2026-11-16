# lesiw.io/tools/analyzers/dotimports

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dotimports.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dotimports)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a dot import.

```go
import . "fmt" // dot import of "fmt"
```

The Import declarations section of the spec says: "If an explicit period (.)
appears instead of a name, all the package's exported identifiers declared in
that package's package block will be declared in the importing source file's
file block and must be accessed without a qualifier"
(https://go.dev/ref/spec#Import_declarations). A name from the imported package
is declared in the file block, and a reader has to find the import to learn
which package supplied it.

Go Code Review Comments allows the form in a test file that cannot be part of
the package under test because of a circular dependency
(https://go.dev/wiki/CodeReviewComments#import-dot). The rule reports such a
file too, because its names still have no qualifier.

Revive's rule can allow packages by configuration. This rule reports every dot
import and has no allow list.

The rule is adapted from revive's dot-imports.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dotimports/cmd/dotimports
go tool dotimports ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dotimports ./...
```
