# lesiw.io/tools/analyzers/impossibleplatform

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/impossibleplatform.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/impossibleplatform)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of `runtime.GOOS` or
`runtime.GOARCH` with a value the file's build constraints rule out.

```go
//go:build windows

package a

func f() bool {
    return runtime.GOOS == "linux" // the file's build constraints rule out GOOS=linux
}
```

Package `runtime` declares `GOOS` and `GOARCH` as constants holding the name of
the target the build was configured for, so the comparison has one answer in
every build of the target the constraint names: `false` for `==` and `true` for
`!=`.

The constraints come from a `//go:build` line before the package clause and
from the file name, the way the go command reads them. A name matching
`*_GOOS`, `*_GOARCH`, or `*_GOOS_GOARCH` states its terms, and the `// +build`
lines of a file that has no `//go:build` line state theirs.

A tag that names neither an operating system nor an architecture can be set or
left clear, so the rule considers both. A comparison that some assignment of
those tags lets through is left alone: with `-tags=alpha`, a file headed
`//go:build windows || alpha` also builds where `GOOS` is linux, so
`runtime.GOOS == "linux"` survives. The rule gives up on a constraint that names
more than ten of those tags, and leaves its comparisons alone.

A tag that another value implies holds for that value as well, since those are
the tags the go command sets: android satisfies `linux`, ios satisfies
`darwin`, illumos satisfies `solaris`, and `unix` holds for every GOOS in the
`unix` list. So `GOOS == "android"` is left alone in a `//go:build linux` file.

A tag can also be set on the command line, so `-tags=windows` builds a file
named `a_windows.go` on a darwin machine, where `GOOS` is darwin. The rule
reads a file name the way the go command reads one, and does not know the tags
a run adds.

The rule reads the files the current build includes. A file the build excludes
is not offered to the analyzer, so a comparison in it waits for a run on a
platform that includes it.

The rule reads `==` and `!=` only, and only a string literal: a comparison
with an ordering operator or against a named constant is left alone. A literal
that names no value the go command knows is left alone as well, which covers a
target added after this rule was written, a fork's target, and a typo.

A `switch` on `GOOS` or `GOARCH` is not a comparison and is left alone.

The fix writes the value the comparison has in the file's builds: `false` for
`==` and `true` for `!=`. It is withheld when a comment sits inside the
comparison, because the edit would delete the comment.

The rule is adapted from staticcheck's SA4032, which reports the same
comparisons without a fix and reads only the left operand.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/impossibleplatform/cmd/impossibleplatform
go tool impossibleplatform ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -impossibleplatform ./...
```
