# lesiw.io/tools/analyzers/dubbitshift

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/dubbitshift.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/dubbitshift)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a shift of a fixed-size typed constant by a
count at least as large as the type's width, or a shift of an unsigned typed
constant whose result is zero below that width.

```go
const n = uint8(1) >> 8 // shift count 8 is at least the 8-bit width of the operand
```

```go
var flag = uint8(1) >> 1 // constant shift by 1 moves every set bit out of the operand
```

The suite's `shift` pass reports a non-constant operand shifted by a constant
count at least the width, and skips a constant operand. This rule applies to
constant operands, including a shift below the width.

The widths come from Go's numeric types
(https://go.dev/ref/spec#Numeric_types). The spec says shifts behave as if the
left operand is shifted once per count, and that there is no upper limit on the
count (https://go.dev/ref/spec#Shift_operators). A count at least as large as
the operand's width leaves no bit of the original value in place. A signed
right shift repeats the sign bit, so its result is zero or -1.

An unsigned typed constant is reported when its result is zero and its operand
is not zero, even where the count is below the width. `uint8(1) >> 1` is zero,
and every set bit it had was shifted out. A zero operand is not reported by the
zero-result clause; the count clause reports it when the count reaches the
width.

Exempt, deliberately:

- A non-constant operand, including the left side of a shift assignment. The
  `shift` pass reports it when the count is a constant at least the width, and
  this rule applies to constant operands only.
- A shift whose count is not constant.
- An untyped operand, which has no width. `1 << 64` is a single wide constant,
  and `1 >> 1` has a zero result.
- A shift whose expression has a platform-dependent type anywhere in it:
  `int`, `uint`, or `uintptr`. This exemption depends on the type, so a named
  constant of type `int` is left alone too. A value of a platform-dependent
  type may depend on the target, and a fix for one target can change the
  source for another.
- A signed shift below the width, including `int8(1) >> 1` and
  `int8(127) >> 7`. The zero-result clause applies to unsigned operands only;
  a signed shift below the width is left alone.
- A shift in a const spec that uses `iota` and is inherited by a later spec.
  One expression node stands for every spec, and the counts differ
  (https://go.dev/ref/spec#Constant_declarations).

A named constant of a fixed-size type can hold a value computed for the target,
which the shift expression does not name. A shift that names one is reported for
the target the analyzer runs against and gets no fix.

The fix replaces the shift with its constant result, as a conversion to the
operand's type: `uint8(0)`.

The fix is withheld for a comment inside the text the edit replaces, for an
enclosing shift whose fix covers it, for an operand or count that names a
constant other than iota, and for a type from a dot import.

The rule is adapted from staticcheck's SA9006, which reports a shift of a
fixed-size integer by a count at least as large as the type width. This rule adds
the zero-result clause below the width.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/dubbitshift/cmd/dubbitshift
go tool dubbitshift ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -dubbitshift ./...
go tool lesiw.io/tools/cmd/fix -dubbitshift ./...
```
