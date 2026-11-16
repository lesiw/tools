# lesiw.io/tools/analyzers/isnilbool

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/isnilbool.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/isnilbool)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of a comparison with a boolean
literal.

```go
func f(a, b int) bool {
    return (a == b) == true // comparison with true; write the inner comparison
}
```

The specification says a comparison yields an untyped boolean value
([Comparison
operators](https://go.dev/ref/spec#Comparison_operators)). The result of
`a == b` is therefore already the boolean that the outer comparison tests.
`(a == b) == true` and `(a == b) != false` have the value of `a == b`. The
literal may be on either side, and either comparison may be parenthesized.

A comparison is left alone when:

- the operator and literal invert the operand, as in `(a == b) == false` or
  `(a == b) != true`. The rewrite would add a negation.
- the operand is not a comparison, as in `ok == true` or
  `(ok && other) == true`.
- `true` or `false` is a name that shadows the predeclared constant.
- the boolean is a named constant, as in `const yes = true`.
- a comment sits in the text the fix deletes. The diagnostic is reported
  without a fix, since the edit would delete the comment.

The fix writes the source text of the inner comparison in place of the outer
comparison. A comment inside the inner comparison is kept.

The rule is adapted from staticcheck's `S1002`, which reports a comparison with
a boolean constant for any boolean operand.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/isnilbool/cmd/isnilbool
go tool isnilbool ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -isnilbool ./...
go tool lesiw.io/tools/cmd/fix -isnilbool ./...
```
