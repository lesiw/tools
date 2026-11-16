# lesiw.io/tools/analyzers/unnecessaryif

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unnecessaryif.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unnecessaryif)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if whose else block contains a single
if.

```go
func f(n int) int {
    if n == 0 {
        return 0
    } else {
        if n == 1 {
            return 1
        }
    }
    return -1
}
```

The IfStmt production of the Go spec admits an IfStmt after else, so the nested
form and the else if form are the same statement. The second removes a pair of
braces and one level of indent.

```go
} else if n == 1 {
```

An else if link has no block of its own, so only a block written after else can
be reported. That block contains one statement, and the statement is an if.

The rule is adapted from go-critic's elseif.

## Not reported

- an else block with a statement besides the if, before it or after it;
- an else block whose single statement is not an if, as a labeled if;
- an empty else block;
- an if with no else.

## Fixes

The fix removes the braces around the nested if and writes else if in their
place. The edit is withheld when a comment lies in the text it deletes:
between the left brace of the else block and the nested if, or between the
nested if and the right brace of the else block. A comment inside the body of
the nested if stays where it is, and the fix still applies.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unnecessaryif/cmd/unnecessaryif
go tool unnecessaryif ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unnecessaryif ./...
go tool lesiw.io/tools/cmd/fix -unnecessaryif ./...
```
