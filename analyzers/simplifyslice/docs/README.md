# lesiw.io/tools/analyzers/simplifyslice

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/simplifyslice.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/simplifyslice)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a slice expression whose high index is the
length of the value.

```go
func f(xs []int) {
    _ = xs[2:len(xs)] // high index can be omitted; it is the length of the value
}
```

The slice expressions section of the spec says that "a missing high index
defaults to the length of the sliced operand", and gives `a[2:]` as the same
expression as `a[2 : len(a)]`
(https://go.dev/ref/spec#Slice_expressions). The [simplify
command](https://pkg.go.dev/cmd/gofmt#hdr-The_simplify_command) of `gofmt`
writes the short form; `vet` and `fix` do not run `gofmt -s`.

Vendored from gopls' `simplifyslice`
(`gopls/internal/analysis/simplifyslice`, `golang.org/x/tools/gopls` v0.24.0),
which is internal and cannot be imported. The rule is upstream's; the code is
adapted to this repository's helpers.

## Checks

The value being sliced is a bare name, and the high index is a call to the
predeclared `len` of that same name. The value may be a slice, an array, a
string, a pointer to an array, a defined type over one of them, or a type
parameter whose type set is one of them.

```go
func f(xs []int) {
    _ = xs[:len(xs)] // high index can be omitted; it is the length of the value
}
```

The value and the argument of `len` are compared by the object each name
resolves to, so a value the file does not declare itself is reported, including
one declared in another file of the package or brought in by a dot import. The
call must be to the predeclared `len`, so a `len` declared in the file or
elsewhere in the package is not a match.

## Not reported

- A high index that is not the length of the value, as in `xs[2:len(ys)]` or
  `xs[2:len(xs[1:])]`.
- A value written as a selector, as in `box.bs[2:len(box.bs)]`. The rule reads
  a bare name on both sides, and a field is not one.
- A parenthesized operand, as in `(xs)[2:len(xs)]`, `xs[2:(len(xs))]`, or
  `xs[2:len((xs))]`. The rule reads identifiers and does not look through
  parentheses.
- A three-index slice, as in `xs[2:len(xs):len(xs)]`, which requires its high
  and max indices.

## Fixes

The fix deletes the high index, so `xs[2:len(xs)]` becomes `xs[2:]` and
`xs[:len(xs)]` becomes `xs[:]`. The low bound is left as written, so
`xs[0:len(xs)]` becomes `xs[0:]`.

It is withheld when the edit would delete a comment, as in
`xs[2:len(/* keep */ xs)]`. The diagnostic is still reported.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/simplifyslice/cmd/simplifyslice
go tool simplifyslice ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -simplifyslice ./...
go tool lesiw.io/tools/cmd/fix -simplifyslice ./...
```
