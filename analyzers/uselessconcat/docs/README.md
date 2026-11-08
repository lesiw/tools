# lesiw.io/tools/analyzers/uselessconcat

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/uselessconcat.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/uselessconcat)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports string literals joined by `+` where the
join fits on one line.

A message split across lines to stay under the limit is a normal way to write
a long string. The `+` is what makes the split possible, so it earns its place
while the line needs it and becomes noise the moment the line fits without it.
Nothing marks that moment: a rename, a shorter message, or a formatter run
that rejoined the operands leaves the `+` behind.

## Checks

### Two literals, one line

```go
var msg = "one " + "two" // string literals joined by + fit on one line
```

The merge is offered as a fix:

```go
var msg = "one two"
```

A join on one line always merges: dropping the + and the quotes can only make
the line shorter, so no width is needed. A join that spans lines moves its text
onto the first line, and that is only reported when a width was set and the
merged line fits it — a line dominated by one string literal is exempt from the
width, so a long message merges too.

Exempt, deliberately:

- A join with an operand that is not a literal.
- A literal holding a newline. Merging it would produce a raw string, which
  is a different shape than the one the author chose.
- A raw string holding a backtick. A raw string cannot hold one, so the merge
  would have to change kind.
- Operands of different kinds. An interpreted and a raw string merge into
  neither one.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/uselessconcat/cmd/uselessconcat
go tool uselessconcat ./...
```

