# lesiw.io/tools/analyzers/blankimports

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/blankimports.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/blankimports)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a blank import in a non-main, non-test file
with no comment naming its path.

```go
package a

import _ "net/http/pprof"
```

The import above is reported: no comment in its declaration contains its path.

The imports section of the spec says, "To import a package solely for its
side-effects (initialization), use the blank identifier as explicit package
name." A blank import triggers the imported package's initialization, and no
declaration from that package appears in the file through it.

A comment explains an import when it contains the import path. The comment may
sit on the import, on the declaration, or after the last import in the
declaration. The path must appear as a whole term: a path followed by a slash
or another path byte is part of a longer path, so
`// net/http/pprof registers handlers` contains `net/http/pprof` but not
`net/http`. A trailing dot, ellipsis, or hyphen ends the term when no path byte
follows it. A path followed by an exported selector, as in
`crypto/rand.SetTestingReader`, names the package, and so does a path in a URL.

A file whose package clause is `main` is left alone, as revive does. A test
file is left alone too; the [Go Code Review
Comments](https://go.dev/wiki/CodeReviewComments#import-blank) allow blank
imports in tests that need them. A blank import of `unsafe` is left alone
whether or not a `//go:linkname` directive is present; the
[compiler documentation](https://pkg.go.dev/cmd/compile) describes the
directive's need for `unsafe`. A blank import of `embed` is left alone in a
file with a `//go:embed` directive; the
[embed package documentation](https://pkg.go.dev/embed) says the directive
needs an import of `embed`.

Revive's rule leaves test files alone, checks only the first blank import of a
contiguous group, and treats any doc or line comment on the import as enough.
This rule also leaves test files alone, but every blank import is read, a
comment must contain the path, and the unsafe exemption is added.

Report only: the repair is a comment or a removal, and only the author knows
which the import is for.

The rule is adapted from revive's `blank-imports`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/blankimports/cmd/blankimports
go tool blankimports ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -blankimports ./...
```
