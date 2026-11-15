# lesiw.io/tools/analyzers/boolexprsimplify

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/boolexprsimplify.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/boolexprsimplify)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a boolean expression with a redundant
negation or comparison.

```go
func f(ok bool) bool {
    if !!ok { // negation of a negation; write the operand
        return true
    }
    return ok != false // comparison with false; write the operand
}
```

The spec says a logical operator yields a result of the same type as its
operands, so `!!ok` is `ok`. A comparison of a boolean value with `true` using
`==`, or with `false` using `!=`, has the value of the operand too.

A comparison yields an untyped boolean value, which any boolean type takes,
while the operand carries its own type. The type checker records the type the
comparison's value was given, and the rule reads that record: a `bool`, or the
untyped value a condition keeps, means the operand can stand in. A comparison
is left alone when:

- its operand's type is not the predeclared `bool`, as in `v == true` for a
  variable of named boolean type `flag`. The rewrite would compile there, and
  the comparison keeps its shape anyway, so a named boolean type is written the
  way its author wrote it;
- the type checker recorded a named boolean type for the comparison, as in
  `v && (b == true)`, where a `bool` would not compile in its place;
- the operand is itself the predeclared `true` or `false`, as in
  `true == true`, which is a constant expression with nothing to restate. A
  name that shadows `true` or `false` is not the constant;
- the constant inverts the operand, as in `ok == false` or `true != ok`, where
  the rewrite would add a negation rather than drop one.

Each fix writes the operand in place of the expression. A negation prints the
operand on one line with the parentheses it already had, so `!(!(a || b)) && c`
becomes `(a || b) && c`; a comparison deletes itself, so `!(ok == true)`
becomes `!(ok)`. Three shapes are reported without a fix: a comparison or a
negation with a comment inside it, whose edit would delete the comment; a
negation whose operand begins with a composite literal, which the edit would
put at the head of an `if` or `for`; and an edit that would write a line past
the width `linelen` sets.

The rule is like go-critic's `boolExprSimplify`, which also inverts a negated
comparison, and like staticcheck's `S1002`, which reports a comparison with a
boolean constant of either value.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/boolexprsimplify/cmd/boolexprsimplify
go tool boolexprsimplify ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -boolexprsimplify ./...
go tool lesiw.io/tools/cmd/fix -boolexprsimplify ./...
```
