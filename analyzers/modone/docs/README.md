# lesiw.io/tools/analyzers/modone

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/modone.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/modone)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a binary remainder operation with a divisor
of one.

```go
func f(n int) int {
    return n % 1 // the remainder is zero when the divisor is one
}
```

The spec defines the remainder as `r = x % y`, where `|r| < |y|`
(https://go.dev/ref/spec#Arithmetic_operators). With `y` equal to one, the
only integer whose magnitude is less than one is zero, so the expression is
zero for every integer `x`.

The rule reports a remainder whose divisor is a constant expression with value
one and no identifier, as in `n % 1`, `n % 0x1`, `n % 1.0`, `n % '\x01'`, or
`n % (1 + 0)`. A divisor that contains an identifier is left alone; the rule
reports only identifier-free constant expressions of value one. The rule covers
only the positive divisor 1, as SA4028 does, so a
divisor of `-1` is left alone. The rule does not report `1 % n`, which is not
always zero. The rule covers binary remainders only, so a compound assignment
such as `n %= 1` is left alone.

The fix replaces `%` with `*` and the divisor with `0`, leaving the left
operand in place. The zero is `'\x00'` when the left operand has the untyped
rune type, and `0` otherwise. An untyped int zero would change the type
inferred for the variable in a short declaration, or the default type of an
untyped constant. Replacing the expression with `0` would skip the left
operand, so a call would not run and a type assertion would not panic, and
would change the type when the operand has a defined type. There is no fix
when a comment is inside the divisor.

The rule is adapted from staticcheck's SA4028.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/modone/cmd/modone
go tool modone ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -modone ./...
go tool lesiw.io/tools/cmd/fix -modone ./...
```
