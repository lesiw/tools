# lesiw.io/tools/analyzers/noopbits

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/noopbits.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/noopbits)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a bitwise operation with a zero literal
or iota zero.

```go
func mask(n int) int {
    return n & 0 // n & 0 always equals 0
}
```

```go
func tag(n int) int {
    return n | 0 // n | 0 always equals n
}
```

In the spec's operator table, `&` is bitwise AND, `|` is bitwise OR, and `^`
is bitwise XOR (https://go.dev/ref/spec#Arithmetic_operators), and constant
expressions are evaluated at compile time
(https://go.dev/ref/spec#Constant_expressions). A zero bit clears the matching
result bit under AND, so `n & 0` is zero. Under OR and XOR a zero bit leaves
the matching bit of the other operand, so `n | 0` and `n ^ 0` are `n`.

The zero operand is a written integer zero, with or without a sign or
parentheses, or an integer iota constant declared in the same package that has
value zero (https://go.dev/ref/spec#Iota). An iota zero is reported because a
flag set that starts at zero often has a member that was meant to be
`1 << iota`. When the constant is iota itself, `1 << iota` appears in the
message; a zero derived from an iota expression with an offset is reported
without that suggestion. Iota zeros are reported without a fix.

The fix drops the operator and the zero operand, leaving the other operand.

`n & 0` is reported without a fix. Its result is zero, and replacing the
expression with an untyped `0` would drop the evaluation of `n`.

The fix is withheld where a comment falls inside the text the edit replaces.

Not reported:

- A zero on the left with a nonzero right operand, as in `0 | n`. A flag set
  often has its zero base first, as in `0x00 | disposal`.
- A constant other operand, as in `Flag | 0` or `5 | 0`. A constant beside a
  zero is often written for symmetry, as a flag payload is.
- A zero operand written as a named constant other than an iota zero, or as a
  constant expression, as in `n | zero` or `n | (2 - 2)`. A zero-valued
  constant is often a base for a flag set, as `O_RDONLY` is.
- A zero written as a floating-point or rune literal, as in `n | 0.0` or
  `n | '\x00'`. This is SA4016's integer-literal test.
- An operand that is not a constant. A variable that holds zero at run time is
  left alone.
- A shift by zero. A shift count of zero is often one case in a series of
  shifts.
- `&^` with zero, as in `n &^ 0`. The rule is limited to SA4016's three
  operators, AND, OR, and XOR, so `&^` is left alone.

The rule is adapted from staticcheck's SA4016
(https://staticcheck.dev/docs/checks/#SA4016). Unlike SA4016, the rule reads
iota zeros from inherited and offset expressions, leaves a constant other
operand alone, and offers a fix for OR and XOR with a literal zero on the
right.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/noopbits/cmd/noopbits
go tool noopbits ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -noopbits ./...
go tool lesiw.io/tools/cmd/fix -noopbits ./...
```
