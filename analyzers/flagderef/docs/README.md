# lesiw.io/tools/analyzers/flagderef

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/flagderef.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/flagderef)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a flag value read before Parse can set it.

```go
package a

import "flag"

var b = *flag.Bool("b", false, "b docs") // flag read before Parse

func f() {
	_ = *flag.String("s", "", "s docs") // flag read before Parse
	_ = flag.Int("n", 0, "n docs")
}
```

flag.Bool and its siblings register a flag and return a pointer to its value.
Before Parse, the pointer points at the default value. The flag the user set is
the one value the program never uses.

The idea comes from go-critic's flagDeref.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/flagderef/cmd/flagderef
go tool flagderef ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -flagderef ./...
```
