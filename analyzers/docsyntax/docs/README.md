# lesiw.io/tools/analyzers/docsyntax

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/docsyntax.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/docsyntax)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports markup syntax in Go comments.

Go has a doc comment syntax of its own, defined by `go/doc/comment` and
described at https://go.dev/doc/comment. A Markdown construct that Go does not
implement is left in the text, so the reader sees its punctuation.

## Checks

```go
// F runs with a `code span` in its doc.   // code span not supported by Go doc syntax
// G is **bold** about it.                 // bold not supported by Go doc syntax
// H links to [the docs](https://x.dev).   // markdown link not supported by Go doc syntax
// ```go                                   // fenced code block not supported by Go doc syntax
// ```                                     // fenced code block not supported by Go doc syntax
```

Go doc syntax has forms of its own, and they are not findings:

- An indented block is a code block, and a backtick belongs there.
- A heading is a line beginning with a number sign and a space.
- A bracketed name with no URL after it is a doc link.
- A line beginning with a dash is a list item.

Report only. The repair is prose.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/docsyntax/cmd/docsyntax
go tool docsyntax ./...
```

