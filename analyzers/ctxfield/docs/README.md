# lesiw.io/tools/analyzers/ctxfield

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/ctxfield.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/ctxfield)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a context.Context in a field.

```go
package a

import "context"

type good struct {
	ctx context.Context // context.Context in a field
}
```

A context has the deadline and the cancellation of one request. A struct that
stores one cannot serve the next request. The field also makes a dependency
implicit, where a parameter would make it explicit.

The idea comes from the containedctx linter.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/ctxfield/cmd/ctxfield
go tool ctxfield ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -ctxfield ./...
```
