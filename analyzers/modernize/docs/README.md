# lesiw.io/tools/analyzers/modernize

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/modernize.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/modernize)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

A suite of `analysis.Analyzer`s for shapes upstream's `modernize` suite does
not cover, run alongside it: a hand-rolled encoding of something the standard
library already does, or two calls where one answers both questions.

The analyzers share a package because they are one kind of thing, not because
they are related.

## Checks

### uuid

A UUID built by formatting hex digits into the canonical shape.

```go
fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b, b, b, b, b) // UUID built by formatting; use a library
fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]) // ditto
```

A UUID is a value with one spelling and one parser, and both live in a
library. Building the text by hand gets the version and variant nibbles wrong,
and it produces a string that no parser will agree with.

Report only. Replacing it is a decision about which library to depend on.

### containscut

A `Contains` check whose answer `Cut` already returns.

```go
if strings.Contains(ref, "@") { // Contains followed by Cut; Cut reports the separator itself
	repo, _, _ := strings.Cut(ref, "@")
	return repo, "", true
}
```

`Cut` reports whether the separator was present, so the `Contains` is a second
scan for a boolean the next line throws away. The idiomatic form asks once:

```go
if repo, _, ok := strings.Cut(ref, "@"); ok {
	return repo, "", true
}
```

Report only. The rewrite moves a declaration into the condition, which is the
author's layout decision.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/modernize/cmd/modernize
go tool modernize ./...
```
