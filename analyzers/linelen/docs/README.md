# lesiw.io/tools/analyzers/linelen

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/linelen.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/linelen)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for line length.

Width is measured in runes after expanding tabs, so a multibyte
character counts once and a tab counts as its display width.
Carriage returns are stripped before measuring. The defaults — a
limit of 79 and a tab width of 4 — are flags.

## Checks

### Lines over the limit

A line wider than the limit is reported with its measured width.
A tab counts four columns by default. A line is not reported when:

- It is an import declaration.
- It fits once the text of the signatures on it is removed, and no body's
  first statement starts on it. A function type is a signature wherever it
  appears, and one is never broken across lines, so wrapping it is not a
  remedy; a line the rest of the statement makes long is still reported, and
  so is the line a body's first statement starts on, because the statement
  can move down.
- It is in a generated file.
- It holds an example's expected output in a test file. That text is
  program output, so wrapping it changes what the test compares.
- One continuous string literal or URL takes up more than half of
  the line's width, ignoring indentation.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/linelen/cmd/linelen
go tool linelen ./... # equivalent to: go tool linelen -len 79 -tab 4 ./...
```
