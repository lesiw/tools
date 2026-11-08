# lesiw.io/tools/analyzers/bookends

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/bookends.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/bookends)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for bracket layout. Three shapes are reported, and each
fix moves the brace rather than the code around it:

- a closing bracket that is not at the indent of its opening bracket;
- an elided-type literal whose opening brace shares a line with its first
  element, so the rows of it start at different columns;
- a table whose rows disagree about their shape, where one row on a line and
  another across lines is neither a table on one line nor a table to read
  down.

## Usage

    go tool lesiw.io/tools/cmd/vet -bookends ./...
    go tool lesiw.io/tools/cmd/fix -bookends ./...

## Checks

### A closer at the wrong indent

A closer on a line of its own is indented to match its opener. A closer
that trails the last item of a list keeps that item's indent instead, so
the two ends of one pair land at different columns.

    xs = append(xs, T{a: 1,
        b: 2}) // closer is not at the indent of its opener

The fix starts a new line before the closer and adds the comma that line
needs. The printer then places the closer under its opener.

- Only a pair that takes a trailing comma is reported. A closer on its own
  line must follow a comma: any other token at the end of a line takes a
  semicolon with it, so an index, a slice, a paren and a type assertion
  cannot break before their closer at all.
- A closer that already starts a line is left alone. The printer places it
  at the opener's indent whatever the source says, so the pair is already
  balanced.
- A parameter list is left alone. A signature stays on one line or wraps
  as its author chose, the same exemption the width check makes.
- A comment between the last item and the closer stays where the author
  put it. The comma goes in front of the comment and the new line goes
  after it.
