# lesiw.io/tools/analyzers/linewrap

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/linewrap.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/linewrap)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` for source lines that wrap where they should not.

It pairs with `lesiw.io/linelen`: `linelen` owns how long a line may be,
`linewrap` owns where a line may be broken.

## Checks

### Statement headers

An `if`, `for`, or `switch` statement keeps its header on a single
line: the opening brace of the body sits on the same physical line as
the keyword that owns it.

```go
if err := longCall( // if header spans 3 lines, exceeds the 1-line limit
    ctx, arg,
); err != nil {
```

Hoist the value to a named variable instead.

Boundaries:

- **`else if`.** Each link in a chain owns its own header. A wrap is
  reported at the `if` keyword of the link that wrapped, and a long
  `else` *body* is never a header wrap.
- **`switch` and type switch.** A wrapped tag expression, a wrapped
  init statement, or a wrapped `x := v.(type)` assignment all push
  the brace down and are reported at the `switch` keyword.
- **`case` clauses.** Not part of the header. A `case` expression
  may wrap freely.
- **Function signatures.** A wrapped signature or function literal is
  not a header wrap; line length is `linelen`'s business.
- **Labels.** A label does not move the report; it lands on the
  keyword.
- **Comments.** A trailing line comment sits after the brace and never
  affects the verdict. A block comment that pushes the brace onto a
  later line is reported, because the header does occupy more than one
  line.
- **Headers that cannot fit.** There is no length carve-out; hoist
  the value to a named variable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/linewrap/cmd/linewrap
go tool linewrap ./...
```

