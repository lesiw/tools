# lesiw.io/tools/analyzers/redundantalias

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/redundantalias.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/redundantalias)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an import alias equal to the declared
package name and the last element of its path.

```go
package a

import (
	fmt "fmt" // redundant import alias "fmt"
	"os"
)

var _ = fmt.Sprint(os.Args)
```

The spec makes the name in the package clause the default when no alias is
written, so an alias equal to that name repeats the default.

A path like `example.com/work/v2` ends in `v2` while its package is called
`task`, and nothing in the import but the alias says the package is `task`, so
the rule leaves `task "example.com/work/v2"` alone. An alias named `v2` for the
same path names nothing the package declares, and removing it would leave every
`v2.X` use unresolved.

The idea comes from revive's redundant-import-alias, which compares the alias
to the last element of the path.

Fixable: the alias is removed, and a comment written between it and the path
goes to the end of the spec.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/redundantalias/cmd/redundantalias
go tool redundantalias ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -redundantalias ./...
go tool lesiw.io/tools/cmd/fix -redundantalias ./...
```
