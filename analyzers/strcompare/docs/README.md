# lesiw.io/tools/analyzers/strcompare

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/strcompare.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/strcompare)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of a `strings.Compare` or
`bytes.Compare` result with a constant.

```go
func textOrdering(a, b string) bool {
    return strings.Compare(a, b) < 0 // comparison through Compare; compare directly
}

func sameBytes(x, y []byte) bool {
    return bytes.Compare(x, y) == 0 // Compare tested against zero; use bytes.Equal
}
```

`strings.Compare` exists for sorting and for three-way comparisons. Against
zero, the operator does the same with less code.

A comparison with `-1` or `1` is reported too, since `Compare` returns one of
those three values: `== -1` is `<`, `!= 1` is `<=`, and `> -1` is `>=`. The
comparison is reported whether the call or the constant comes first, and the
fix compares the two arguments in the order the call has them, so
`0 > Compare(a, b)` becomes `a < b`.

A byte slice has no comparison operator, so `bytes.Compare` keeps its ordering,
and only its zero test has a shorter form. `!= 0` becomes `!bytes.Equal`, and
the call keeps the package qualifier it was written with. A comparison whose
edit would delete a comment, or whose arguments do not print on one line, is
reported without a fix.

Not reported:

- A `bytes.Compare` test against anything but zero, as
  `bytes.Compare(x, y) == 1`.
- A comparison whose answer does not depend on the strings, as
  `strings.Compare(a, b) == 5`, `strings.Compare(a, b) > 1` or
  `strings.Compare(a, b) >= -1`. `Compare` returns -1, 0, or 1, so the test has
  the same answer wherever it is evaluated.
- A test against a named constant, since the fix would remove its use.
- A comparison through a value with its own `Compare` method, a dot import, or
  a package name that another value shadows.
- A result held in a variable and compared there.

The rule is inspired by go-critic's `stringsCompare`. The `bytes.Compare` arm
is adapted from staticcheck's `S1004`, which reads only the comparison with the
call and the literal zero in that order.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/strcompare/cmd/strcompare
go tool strcompare ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -strcompare ./...
go tool lesiw.io/tools/cmd/fix -strcompare ./...
```
