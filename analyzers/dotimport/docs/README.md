# lesiw.io/tools/analyzers/dotimport

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dotimport.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dotimport)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a dot import in a file that is not a test.

```go
package a

import . "strings"

func f(s string) bool {
    return Contains(s, "x")
}
```

The dot form declares every exported identifier in the imported package's
package block in the file block, so the file does not show where a name is
defined. The Code Review Comments'
[Import Dot](https://go.dev/wiki/CodeReviewComments#import-dot) allows the form
in a test that cannot be part of the package it tests. Every test file is
exempt, which is wider than the cited case.

Inspired by revive's dot-imports.

The fix removes the dot and qualifies every use of a package-level identifier
with the name the file binds to the path, using an existing import or the
package's own name. It is offered when that name reaches the imported package
at every use. An in-place conversion is not offered when a second dot-imported
package has the same name, or when the dot and path are on different lines.
Removing a dot import is not offered when its declaration carries a doc comment
or the import shares a line with another import. The fix is also not offered
when a qualifier or a moved comment would push a line past the width, or when a
comment sits between the dot and the path. A single-line trailing comment moves
to the import that stays when that import is alone on its line and has no
comment; any other trailing comment blocks the fix.

A dot import of `internal/types/errors` is left alone: `go/types` and
`cmd/compile/internal/types2` use that package for their error codes.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dotimport/cmd/dotimport
go tool dotimport ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dotimport ./...
go tool lesiw.io/tools/cmd/fix -dotimport ./...
```

`-dotimport.allowed` takes a comma-separated list of import paths and adds them
to the default list, which contains `internal/types/errors`. The flag can be
repeated.

```sh
go tool lesiw.io/tools/cmd/vet \
    -dotimport.allowed=example.com/constants ./...
```
