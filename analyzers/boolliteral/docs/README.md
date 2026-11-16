# lesiw.io/tools/analyzers/boolliteral

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/boolliteral.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/boolliteral)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison with a boolean literal.

```go
func f(ok bool) bool {
    return ok == true // comparison with true; write the operand
}

func g(ok bool) bool {
    return ok == false // comparison with false; write the operand negated
}
```

Go's specification says a comparison yields an untyped boolean value
(https://go.dev/ref/spec#Comparison_operators), and the logical operators yield
a result of the same type as their operands
(https://go.dev/ref/spec#Logical_operators). So `ok == true` and `ok != false`
have the value of `ok`; `ok == false` and `ok != true` have the value of `!ok`.
The rule reports all four, with the literal on either side.

The comparison is replaced by the operand, adding `!` where the comparison
inverts the operand. A negated binary operand is parenthesized, so
`a < b == false` becomes `!(a < b)`.

A comparison is left alone when:

- the operand's type is not boolean, as in `x == true` for a value of type
  `any`. With `x` of interface type, the comparison succeeds only when the
  dynamic type is bool and the value is true; the operand alone does not imply
  both;
- both operands are boolean literals, as in `true == false`, because neither
  side can replace the comparison;
- neither side is the predeclared `true` or `false`, as when a name shadows
  either literal;
- the comparison has an untyped boolean result and the operand is neither an
  untyped boolean value nor the predeclared `bool` or an alias of it. A named
  boolean type is written the way its author wrote it. The rule uses the
  comparison's recorded type and does not separately inspect what the parent
  would accept;
- the comparison is inside a constant declaration, has an untyped result, and
  its operand is typed, because the operand's type would become the constant's
  type;
- the comparison has a typed result and the operand is not assignable to it.
  An operand of named boolean type is not assignable to `bool`;
- a comment sits in the text the fix removes;
- a rewritten line would be wider than the `linelen` width, unless a line it
  replaces is already past the limit.

The rule is adapted from revive's `bool-literal-in-expr`.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/boolliteral/cmd/boolliteral
go tool boolliteral ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -boolliteral ./...
go tool lesiw.io/tools/cmd/fix -boolliteral ./...
```
