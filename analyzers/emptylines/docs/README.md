# lesiw.io/tools/analyzers/emptylines

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/emptylines.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/emptylines)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&label=discord&logoColor=white&color=5865F2)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports blank lines that separate nothing inside a
block.

```go
func leading(n int) { // blank line at the start of a block

    println(n)
}
```

```go
func run(n int) {
    println(n) // more than one blank line in a row


    println(n)
}
```

The rule reports two shapes: a blank line at either edge of a block, and a run
of two or more blank lines inside one.

An edge is the line directly below the block's opening brace, or directly above
its closing brace, unless that brace's line holds a statement. A blank line on
an edge is reported whatever follows it, a statement or a comment.

Both shapes are inside an `*ast.BlockStmt`: a function body, a func literal's
body, the body of an if, for (range included), switch, or select, and a bare
block. A case or comm clause body is not a block, so the only edges are the
braces of the switch or select.

An edge line separates nothing: the block has no statement on the far side of
it. A run keeps one line, and every line after the first is extra, since a
single blank line already separates the content above it from the content
below. gofmt keeps a blank line at an edge, and it collapses a run to one blank
line, so a run survives only in a file nothing has formatted.

A run is reported wherever it lies inside a block, including inside a struct
body or a composite literal the block encloses. The same run in a top-level
type or literal is outside every block and is left alone.

Not reported, and why:

- a single blank line between two pieces of content inside the block, which is
  the separation a blank line is for;
- a blank line inside a comment or a string, which is text rather than layout;
- a single blank line in a struct body, an interface body, or a composite
  literal, whose braces open no block;
- a blank line outside every block: between two declarations, or inside a
  top-level struct, interface, or composite literal.

The rule is adapted from revive's `empty-lines`, which reports a blank line at
each edge of a block and nothing between them.

Fixable: a blank line at an edge is removed, and a run inside the block keeps
its first line.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/emptylines/cmd/emptylines
go tool emptylines ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -emptylines ./...
go tool lesiw.io/tools/cmd/fix -emptylines ./...
```
