# lesiw.io/tools/analyzers/unsignedcmp

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unsignedcmp.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unsignedcmp)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of an unsigned value against
a limit of its type: the least value it holds, or the greatest value it holds.

```go
func below(n uint8) bool {
    return n < 0 // uint8 is never negative; this is always false
}
```

An unsigned value is never negative, so `n < 0` and `0 > n` are always false,
and `n >= 0` and `0 <= n` are always true. The same holds at the greatest
value: for a `uint8`, `n > 255` and `255 < n` are always false, and `n <= 255`
and `255 >= n` are always true.

A comparison is reported only when the bound is one of those two limits.
`n <= 0` and `n > 0` compare `n` with zero, and `n < 255` and `n >= 255`
compare it with 255, so the result depends on the value of `n`.

The width comes from the target, so only the zero bound is reported for a
`uint` and a `uintptr`. A type parameter is left alone: its underlying type is
its constraint, which is an interface.

A bound can hold one number on a 64-bit build and another on a 32-bit one, and
a constant of some name can hold another value in a file that a different build
compiles. A bound is reported only when it is written where the comparison is:
a literal, a conversion to a `uint8`, `uint16`, `uint32`, or `uint64`, or one of
the `math` package's maxima for them. `255`, `uint8(255)`, and `math.MaxUint8`
are reported for a `uint8`; `top` and `uint8(uintptr(255))` are not.

## Fixes

Where the comparison is the whole condition of an `if` statement, the fix
deletes the branch that cannot run. Where the comparison is one side of a
conjunction or a disjunction, the fix deletes that side when the rest of the
expression has the same value: `false || rest` and `true && rest` both have the
value of `rest`. Otherwise the fix writes `true` or `false` in place of the
comparison.

```go
func guard(n uint8, ok bool) {
    if n < 0 || ok { // uint8 is never negative; this is always false
        return
    }
}

func valid(n uint8) bool {
    return n < 0 // uint8 is never negative; this is always false
}
```

The first fix gives `if ok {`, and the second gives `return false`. Where the
branch always runs, the fix writes the body without the condition, and the
braces stay because they scope the names the body declares; `unnecessaryblock`
deletes them afterwards when the body declares nothing.

No fix is offered when:

- the comparison is in a conjunction or disjunction where the other side gives
  its value, in a loop or switch header, in a case clause, or in a literal's
  key;
- the `if` statement has an init statement, whose effect the fix would delete;
- the body might not fall through, so the statements after the `if` could be
  unreachable;
- reading the operand has an effect, or can panic: a call that is not a
  conversion or a builtin that only reads its arguments, a receive, the address
  of a composite literal, a dereference, an index, a slice expression, a type
  assertion, a division or remainder, or a field reached through a pointer;
- deleting the statement would leave its label with no statement, or the
  deleted code holds a labeled jump, which may be the only jump its label has;
- a comment sits inside the text the fix rewrites;
- deleting the code would leave a variable that nothing reads;
- another finding's edit covers the same text.

Adapted from staticcheck's `SA4003`, which also reports the bounds of a signed
type.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unsignedcmp/cmd/unsignedcmp
go tool unsignedcmp ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unsignedcmp ./...
go tool lesiw.io/tools/cmd/fix -unsignedcmp ./...
```
