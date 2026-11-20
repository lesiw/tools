# lesiw.io/tools/analyzers/simplifyrange

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/simplifyrange.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/simplifyrange)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range clause that writes the blank
identifier instead of leaving it out.

```go
func f(xs []string) {
    for _ = range xs { // range clause can be written without the blank
    }
}
```

The For statements section of the Go spec says that "if the last iteration
variable is the blank identifier, the range clause is equivalent to the same
clause without that identifier" (https://go.dev/ref/spec#For_statements). The
simplify command of `gofmt` writes the same clause without the blank; `vet` and
`fix` do not run `gofmt -s`, so the analyzer reports the clause instead.

Vendored from gopls' `simplifyrange`
(`gopls/internal/analysis/simplifyrange`, golang.org/x/tools/gopls v0.24.0),
which is internal and cannot be imported. The rule is upstream's; the code is
adapted to this repository's helpers.

## Checks

A clause whose value is blank is reported, under `:=` and under `=`.

```go
for key, _ := range xs { // range clause can be written without the blank
}

for key, _ = range xs { // range clause can be written without the blank
}
```

A clause whose key and value are both blank is reported under `=`.

```go
for _, _ = range xs { // range clause can be written without the blank
}
```

A clause whose only variable is a blank key is reported under `=`.

```go
for _ = range xs { // range clause can be written without the blank
}
```

Left alone, for example:

- `for range xs`, which has no iteration variable.
- `for key := range xs` and `for key = range xs`, whose key is not blank.
- `for _, value := range xs` and `for _, value = range xs`, whose value is not
  blank.
- `for key, value := range xs`, whose two variables are not blank.
- `for (_) = range xs` and `for _, (_) = range xs`, which write the blank
  inside parentheses. The rule reads an identifier, and a parenthesized blank
  is not one. A parenthesized key with a blank value, as in
  `for (_), _ = range xs`, is reported.

## Fixes

The fix writes the clause without the blank, so that `for key, _ := range xs`
becomes `for key := range xs` and `for _ = range xs` becomes `for range xs`.
When both variables are blank, both are removed, so `for _, _ = range xs`
becomes `for range xs`.

The fix is withheld when the edit would delete a comment.

It is withheld too when the clause ranges over an array or a pointer to an
array and dropping the value would stop evaluating the range expression. The
spec says the range expression is not evaluated when at most one iteration
variable is present and `x` or `len(x)` is a constant, so `for _, _ = range
*point` panics on a nil pointer and `for range *point` does not. The blank is
still unnecessary, so the diagnostic remains. Upstream offers the edit here;
this analyzer leaves the program alone.

```go
for _, _ = range *point { // range clause can be written without the blank
}
```

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/simplifyrange/cmd/simplifyrange
go tool simplifyrange ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -simplifyrange ./...
go tool lesiw.io/tools/cmd/fix -simplifyrange ./...
```
