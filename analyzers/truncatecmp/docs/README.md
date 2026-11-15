# lesiw.io/tools/analyzers/truncatecmp

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/truncatecmp.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/truncatecmp)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison through a narrowing integer
conversion.

```go
func below(wide int64, bound int32) bool {
    return int32(wide) < bound // comparison truncates int64 to int32
}
```

Go's specification says an integer conversion is truncated to fit in the
result type's size, so the conversion above keeps the low 32 bits of `wide`.
Comparing both values in the wide type is exact while the two types share a
signedness:

```go
return wide < int64(bound)
```

Where the wide type is unsigned and the narrow one signed, the conversion maps
a negative value to a large positive one, and the two comparisons are
different programs.

The rule examines only a conversion that is a whole operand, and leaves alone:

- a conversion whose operand or target is not an integer;
- a conversion of a constant, because the compiler requires a constant to be
  representable in the target type;
- a conversion whose operand or target is a type parameter, because a type
  parameter has no single size.

A comparison with a truncating conversion on both operands is reported once. A
value narrowed before the comparison is left alone, because the rule does not
follow values.

The rule is adapted from go-critic's truncateCmp, which reads conversions to six
named target types, and by default skips a conversion from `int`, `uint` or
`uintptr`, a comparison whose two operands differ in signedness, a comparison
against a literal, and a comparison of two conversions. This rule reads every
integer type and skips none of those.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/truncatecmp/cmd/truncatecmp
go tool truncatecmp ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -truncatecmp ./...
```
