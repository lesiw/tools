# lesiw.io/tools/analyzers/textblock

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/textblock.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/textblock)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a text block built a line at a time from
string constants.

```go
func text() string {
    return "alpha\n" + // text block built a line at a time; use a raw string literal
        "\n" +
        "beta\n" +
        "gamma\n"
}
```

The same text as one raw literal:

```go
func text() string {
    return `alpha

beta
gamma
`
}
```

## Checks

The analyzer reports two shapes:

- A `strings.Join` call whose first argument is a composite literal of two or
  more string literals and whose separator is the constant `"\n"`.
- A chain of four or more string literal operands joined by `+`, where every
  operand but the last ends with a newline. The chain is reported once, at its
  outermost `+`, so a constant prefix of a longer chain is not reported on its
  own.

The call has to resolve to `strings.Join`. A method named `Join`, a local
function, or a variable holding `strings.Join` is left alone. A join of one
part is not a block. `strconcat` leaves a newline join of string literals that
a raw literal can hold to this rule, so the two do not conflict.

## Exemptions

The spec says a raw string literal "may contain any character except the back
quote" (https://go.dev/ref/spec#String_literals). A block with a back quote in
any line stays as it is. A carriage return is dropped from a raw literal's
value, and a NUL or invalid UTF-8 byte is not source text, so a block holding
one of those stays as it is too.

A keyed slice literal is left alone, because the keys decide the order. A
named constant element or operand is left alone in either shape, because the
line is written elsewhere. A chain whose first operand holds only newlines and
whose last operand does not end with a newline joins a newline to a suffix. It
is a fragment rather than a block, so it is left alone.

## Report only

The rewrite preserves the value, but it moves every line to the source's
margin, and a line whose leading whitespace is part of the generated text is a
case the author has to check. A comment between the parts is also lost,
because a raw literal cannot hold it. The analyzer does not suggest an edit.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/textblock/cmd/textblock
go tool textblock ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -textblock ./...
```
