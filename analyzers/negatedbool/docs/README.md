# lesiw.io/tools/analyzers/negatedbool

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/negatedbool.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/negatedbool)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a boolean expression that a negation makes
constant or redundant.

```go
func f(ok bool) bool {
    return !ok && ok // the conjunction of an expression and its negation is always false
}

func g(ok bool) bool {
    return !ok || ok // the disjunction of an expression and its negation is always true
}

func h(ok bool) bool {
    return !!ok // negating a boolean twice has no effect
}
```

The logical operators section of the spec defines `!x` as the inverse of `x`,
`x && y` as false where either operand is false, and `x || y` as true where
either operand is true (https://go.dev/ref/spec#Logical_operators). So
`!ok && ok` is false and `!ok || ok` is true whatever `ok` is. The same section
defines the second `!` in `!!ok` as the inverse of the first, which leaves
`ok`, and says logical operators yield a result of the same type as their
operands, so the replacement preserves a defined boolean type or type
parameter.

A conjunction or disjunction is reported when one immediate operand is a
negation and the other is a name, a constant expression, or a negation of one
with the same text, and parentheses around either one do not matter. A larger
operand is left alone, even when it looks stable. A double negation reads its
operand once and is reported whatever the effect.

The fix writes the operand in place of a double negation, so `!!ok` becomes
`ok` and `!!!ok` becomes `!ok`. A conjunction or disjunction is replaced with
its constant value only when the operand is a constant with no variable
reference or a parameter or result variable, its type is the predeclared
`bool`, an alias of it, or an untyped boolean, and the replacement is the
predeclared `false` or `true` in scope at the edit. A local variable is
report-only because dropping its last read would make it unused. A package
variable is report-only because the edit can reorder package initialization
when the expression appears in an initializer. A defined boolean type is
report-only because an untyped constant can change generic inference and short
declaration types. A constant from another package is report-only so the edit
does not leave an import unused.

The fix is withheld when a comment sits inside the text it would replace. It is
also withheld when the edit would make the line longer than the width `linelen`
sets. A double negation whose operand begins with a composite literal after its
prefix operators, selectors, calls, and similar gets one more pair of
parentheses, so the operand still parses at the head of an `if`, `for`, or
`switch`.

The rule is adapted from staticcheck's SA4013, which reports a double negation
(https://staticcheck.dev/docs/checks/#SA4013).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/negatedbool/cmd/negatedbool
go tool negatedbool ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -negatedbool ./...
go tool lesiw.io/tools/cmd/fix -negatedbool ./...
```
