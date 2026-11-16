# lesiw.io/tools/analyzers/indexcontains

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/indexcontains.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/indexcontains)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a test of a `strings.Index` or
`bytes.Index` result against `-1`.

```go
func hasText(s, sub string) bool {
    return strings.Index(s, sub) != -1 // Index tested against -1; use Contains
}
```

The `strings` package documents `Index` as returning "the index of the first
instance of substr in s, or -1 if substr is not present in s", and `Contains`
as reporting "whether substr is within s". `bytes` documents the same pair for
byte slices. `strings.Index(s, sub) != -1` is true exactly when
`strings.Contains(s, sub)` is true, and `== -1` is true exactly when it is
false.

A comparison with `-1` is reported on either side of the operator. With the
call on the left, `!=` and `>` test for presence, while `==` and `<=` test for
absence. Swapping the operands swaps the operator: `-1 < ...` tests for
presence and `-1 >= ...` tests for absence.

Not reported:

- A comparison with another value, or with a constant expression rather than
  the literal `-1`. A named constant is left alone even when its value is
  `-1`, because the fix would remove its use.
- A comparison that is always true or false, as `strings.Index(s, sub) >= -1`.
- Any other `Index` function, such as `IndexAny`, `IndexRune`, or `IndexByte`.
- A call through a dot import, a function value, or a value that shadows either
  package name.

The rule is adapted from staticcheck's S1003, which also covers `IndexAny`,
`IndexRune`, and the comparisons with zero.

Fixable. The fix replaces `Index` with `Contains` and removes the comparison,
adding a negation where the substring is absent. It is withheld when a comment
sits in the comparison text the rewrite removes.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/indexcontains/cmd/indexcontains
go tool indexcontains ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -indexcontains ./...
go tool lesiw.io/tools/cmd/fix -indexcontains ./...
```
