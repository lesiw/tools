# lesiw.io/tools/analyzers/embeddirective

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/embeddirective.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/embeddirective)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `//go:embed` directive the compiler
rejects. A file with a directive and no `embed` import does not compile, and
neither does a directive that does not precede one variable of type
`string`, `[]byte` or `embed.FS` with no value.

```go
//go:embed static/index.html
var pages []string // must be of type string, []byte or embed.FS
```

The [embed package](https://pkg.go.dev/embed) says the directive "must
immediately precede a line containing the declaration of a single variable"
and "the type of the variable must be a string type, or a slice of a byte type,
or FS (or an alias of FS)".

If a file uses a directive without importing `embed` it does not compile, so the
analyzer runs on a package that fails to type-check. When the variable's type
does not name the package, nothing in the file refers to it, so the import takes
the blank identifier:

```go
import _ "embed"
```

The analyzer accepts `string`, `[]byte`, `embed.FS`, a defined type or alias
whose underlying type is `string` or `[]byte`, and an alias of `embed.FS`. A
type defined from `embed.FS` is reported, which the compiler rejects too. A
slice of a defined byte type is reported, which the embed package's doc allows
and the compiler accepts: upstream reports it, and this keeps upstream's rule.

A directive above a `var` group is not reported when the group's first variable
qualifies, even though the compiler rejects the file.

Fixable: the missing import is added, as `_ "embed"` or as `"embed"` when the
file names the package, and a constant initializer is deleted. These are
reported without a fix:

- a misplaced directive, which needs a move;
- several names, which need a choice of which to keep;
- a type that cannot hold the files, which needs a choice of type;
- a declaration with no type, which would declare nothing once the value goes;
- an initializer that may do work when it is evaluated;
- an initializer with a comment inside the text the edit would delete.

The rule and the code are copied from gopls' `embed`
(`gopls/internal/analysis/embeddirective`, `golang.org/x/tools/gopls` v0.24.0),
which is internal and cannot be imported. The import fix is written against this
repository's `internal/imports`, and the fix that drops an initializer is added
here. Upstream names the analyzer `embed`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/embeddirective/cmd/embeddirective
go tool embeddirective ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -embeddirective ./...
go tool lesiw.io/tools/cmd/fix -embeddirective ./...
```
