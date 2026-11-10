# lesiw.io/tools/analyzers/dupimport

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dupimport.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dupimport)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports one package imported twice under different names.

```go
package a

import (
	"bytes"
	printing "bytes" // bytes is imported more than once
	"strings"
)

func f() {
	_ = bytes.Buffer{}
	_ = printing.Buffer{}
	_ = strings.TrimSpace("")
}
```

Two names for one package give every use site a choice. A reader has to check
that both names are the same package. A blank import of a path the file already
imports is the go:linkname idiom, and the check leaves it alone.

The idea comes from go-critic's dupImport.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dupimport/cmd/dupimport
go tool dupimport ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dupimport ./...
```
