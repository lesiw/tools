# lesiw.io/tools/analyzers/builtinbound

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/builtinbound.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/builtinbound)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of `len` or `cap` with zero
that is always true or always false.

```go
func f(xs []int) bool {
    return len(xs) < 0 // len is never negative; this is always false
}
```

The Length and capacity section of the spec gives `0 <= len(s) <= cap(s)` for a
slice, and gives the length of a string, a map and a channel as a count
(<https://go.dev/ref/spec#Length_and_capacity>). Neither builtin returns a
negative number, so the answer does not depend on the operand: `len(x) < 0` and
`0 > len(x)` are always false, and `len(x) >= 0` and `0 <= len(x)` are always
true. `cap` is reported the same way.

The zero must be the literal `0`, and the call must name the builtin, so a local
`len` is left alone. The other comparisons against zero — `len(x) == 0`,
`len(x) > 0`, `len(x) <= 0`, `0 == len(x)`, `0 < len(x)`, and `0 >= len(x)` —
test whether the operand is empty. [`emptystring`](../../emptystring/docs)
rewrites them to a comparison with `""` when the operand is a string.

Fixable: the comparison becomes `false` or `true`. Replacing it drops the
operand's evaluation, so the fix is withheld where that would run code, panic,
or leave a local variable unread, and where a comment sits inside the
comparison.

Adapted from staticcheck's `SA4024`, which reports the comparisons that are
always false for `len` and `cap`, and from go-critic's `sloppyLen`, which
reports the ones that are always true for `len`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/builtinbound/cmd/builtinbound
go tool builtinbound ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -builtinbound ./...
go tool lesiw.io/tools/cmd/fix -builtinbound ./...
```
