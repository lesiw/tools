# lesiw.io/tools/analyzers/elseif

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/elseif.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/elseif)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if nested in an else block.

```go
func f(n int) int {
    if n == 0 {
        return 0
    } else { // if nested in an else; write else if
        if n == 1 {
            return 1
        }
    }
    return -1
}
```

The nested if is the only statement in the else. The braces and the extra
indent add nothing.

```go
} else if n == 1 {
```

The rule is like go-critic's elseif.

## Fixes

The fix removes the braces and joins the two lines.

- A comment on the else line stays at the end of the `else if` line. When that
  line would exceed linelen's limit, the comment moves into the body instead,
  since the fix would otherwise leave a line that linelen reports. A line that
  the join alone takes past the limit is left as it is, and a run that sets no
  width keeps the comment on the line.
- A comment on its own line before the nested if moves into the body. The
  comment would sit between `else` and `if`, which breaks the `else if` across
  two lines. A header that already spans lines has no single line to end, so
  its comment moves too.

A comment inside the nested if is outside the edit and stays where it is.
Comments that move into the body keep their order in the source, and they sit
above the branch's first statement, which is what they describe there.

The fix is withheld when a carried comment cannot stay inside the statement. A
comment that follows the nested if has only the else block's closing line,
which the fix deletes. A directive, such as `//line table.go:1`, `//nolint`, or
`//ignore`, is a comment with no space after its slashes, and a tool reads it at
its position.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/elseif/cmd/elseif
go tool elseif ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -elseif ./...
go tool lesiw.io/tools/cmd/fix -elseif ./...
```
