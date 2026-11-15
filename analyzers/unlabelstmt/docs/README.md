# lesiw.io/tools/analyzers/unlabelstmt

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unlabelstmt.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unlabelstmt)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a redundant label.

```go
func f(xs []int) {
outer: // redundant label outer
    for range xs {
        break outer
    }
}
```

A bare break leaves the innermost for, switch, or select, and a bare continue
begins the next iteration of the innermost for. The analyzer reports a label
when no goto names it and every break or continue that names it would apply to
the same statement without the name.

The blank label is reported too: nothing can name it. The compiler requires a
use for every other label.

The idea comes from go-critic's unlabelStmt.

Fixable: the whole line goes when the label is alone on it, and each break or
continue that names it becomes bare.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unlabelstmt/cmd/unlabelstmt
go tool unlabelstmt ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unlabelstmt ./...
go tool lesiw.io/tools/cmd/fix -unlabelstmt ./...
```
