# lesiw.io/tools/analyzers/noopdirective

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/noopdirective.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/noopdirective)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comment that reads as a `//go:`
directive but for whitespace after the slashes.

```go
// go:noinline
func f() {}
```

[The Compiler Directives section of
`cmd/compile`](https://pkg.go.dev/cmd/compile#hdr-Compiler_Directives) says each
directive "must be placed its own line, with only leading spaces and tabs
allowed before the comment, and there must be no space between the comment
opening and the name of the directive, to distinguish it from a regular
comment". A line that separates `//` from `go:` with whitespace is an ordinary
comment, so a directive written that way does nothing: `// go:build linux`
constrains nothing, and `// go:embed name` embeds nothing.

The rule reads a comment before the package clause and a comment that documents
a func, var, const, or type declaration or spec, which gofmt leaves indented
inside a `var`, `const`, or `type` block. A comment inside a function body is
left alone. A `go:` directive is a `//` comment, so a block comment does not
qualify, and the rule reads only a name that begins with a lowercase letter,
which leaves `// go: two words about Go` and `// go:Build linux` alone.

A space is also written on purpose, to keep a directive in the file without
running it, and a doc comment can wrap so that a line begins with a directive
name. The rule cannot tell either shape from a typo: a line meant to stay that
way takes an `//ignore:noopdirective` comment above it.

Report only: removing the whitespace makes the directive take effect, and the
effect is not always the one the author wanted. `// go:build` may exclude the
file it heads, and `// go:embed` fails to compile when its pattern matches no
file.

The rule is adapted from staticcheck's `SA9009`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/noopdirective/cmd/noopdirective
go tool noopdirective ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -noopdirective ./...
```
