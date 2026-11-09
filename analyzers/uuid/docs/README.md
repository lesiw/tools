# lesiw.io/tools/analyzers/uuid

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/uuid.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/uuid)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a UUID built by formatting hex digits into
the canonical shape.

A UUID is a value with one spelling and one parser, and both live in a
library. Building the text by hand gets the version and variant nibbles wrong,
and it produces a string that no parser will agree with.

```go
fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b, b, b, b, b) // UUID built by formatting; use a library
fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]) // ditto
```

Report only. Replacing it is a decision about which library to depend on.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/uuid/cmd/uuid
go tool uuid ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -uuid ./...
go tool lesiw.io/tools/cmd/fix -uuid ./...
```
