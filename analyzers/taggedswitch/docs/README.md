# lesiw.io/tools/analyzers/taggedswitch

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/taggedswitch.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/taggedswitch)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an untagged switch that compares one
expression with a series of values.

```go
switch { // switch without a tag; write switch n
case n == 1 || n == 2:
}
```

By the spec's expression switches section
(https://go.dev/ref/spec#Expression_switches), an expression switch compares its
expression with each case expression for equality. An untagged switch that
compares the same expression in every case repeats that expression once per
comparison, and the tag names it once:

```go
switch n {
case 1, 2:
}
```

Every case expression in every clause must be an `==` comparison, or an `||`
chain of them, with the same expression as the left operand. One clause that
holds anything else, such as an `&&`, or whose left operand is a constant or
`nil`, keeps the rule from reporting the switch: the tag would name the
constant and the cases would name the expression. `yoda` handles a literal or
`nil` on the left, and a named constant on the left is left alone. A switch
that compares `n` in one clause and `m` in another is not reported either.

An expression is the same in two clauses when it prints the same and reading it
twice gives the same value, which for two names in one switch means they name
the same objects. A tag or a case value that can give a different answer when
read again stops the report. In the untagged form the tag is read once per
comparison, so code that runs between two reads can change the second.

Fixable: the tag is written and each comparison is replaced with the value it
tests against, so `case n == 1 || n == 2:` becomes `case 1, 2:`. Redundant
parentheses around a value are dropped. The tag is parenthesized when a switch
header cannot begin with it, as in `switch (item{}.n)`. The fix is withheld,
and the switch is still reported, when:

- A comment lies between a clause's first case expression and its colon. The
  rewrite replaces that span, and the comment would go with it. A comment after
  the colon stays.
- Two of the values are the same constant, compared as the type the switch
  converts each one to, so `1` and `1.00000001` are one value in a `float32`
  switch. A tagged switch rejects a duplicate case value, where the comparisons
  it replaces were expressions the compiler does not check for repeats.
- The rewrite would take a line past the width, as it does when a clause broken
  across lines becomes one `case` line.
- The tag has no one-line form, as a tag holding a raw string with a newline
  does not. Writing it in the header would move the switch's brace to another
  line.

The rule is adapted from staticcheck's `QF1002`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/taggedswitch/cmd/taggedswitch
go tool taggedswitch ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -taggedswitch ./...
go tool lesiw.io/tools/cmd/fix -taggedswitch ./...
```
