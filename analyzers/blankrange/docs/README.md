# lesiw.io/tools/analyzers/blankrange

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/blankrange.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/blankrange)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range clause with an unnecessary blank
identifier.

```go
func f(xs []int) {
    for i, _ := range xs { // blank identifier in range clause is unnecessary
        use(i)
    }
}
```

The For statements section of the Go spec says that "if the last iteration
variable is the blank identifier, the range clause is equivalent to the same
clause without that identifier" (https://go.dev/ref/spec#For_statements). The
same section says the range expression is not evaluated when at most one
iteration variable is present and `x` or `len(x)` is constant. Removing a
second variable from a clause over an array or a pointer to an array can
therefore skip an evaluation. When removing the blank is not safe, the clause
is not reported. `gofmt -s` rewrites the same clause, though a parenthesized
blank is unchanged; it also rewrites the array clauses this check does not
report. vet and fix do not apply `gofmt -s`.

The rule is inspired by revive's `range`.

## Checks

A clause whose second variable is blank is reported under either `:=` or `=`.

```go
for i, _ := range xs { // blank identifier in range clause is unnecessary
}

for i, _ = range xs { // blank identifier in range clause is unnecessary
}
```

It is reported too when the first and only variable is blank. The short
declaration requires a new variable, so that clause is written with `=`.
Parentheses around a blank do not change the report; `for (_) = range xs` is
reported too.

```go
for _ = range xs { // blank identifier in range clause is unnecessary
}
```

Both variables can be blank under `=`.

```go
for _, _ = range xs { // blank identifier in range clause is unnecessary
}
```

The key may be an addressable expression, such as `dst[0]`, or a map index.

```go
for dst[0], _ = range xs { // blank identifier in range clause is unnecessary
}
```

Left alone, for example:

- `for range xs`, which has no iteration variable.
- `for i := range xs` and `for i = range xs`, whose key is not blank.
- `for _, v := range xs` and `for _, v = range xs`, whose last variable is
  not blank.
- `for i, v := range xs` and `for i, v = range xs`, whose two variables are
  not blank.
- `for i, _ := range *p`, `for _, _ = range *p`, `for i, _ := range [3]int(s)`,
  and `for i, _ := range o.a`, where `a` is promoted through a pointer.
  Removing the blank from these clauses would skip an evaluation that can
  panic or have a side effect.

## Fixes

The fix writes the clause without the blank, so that `for i, _ := range xs`
becomes `for i := range xs` and `for _ = range xs` becomes `for range xs`. When
both variables are blank, both are removed, so `for _, _ = range xs` becomes
`for range xs`.

The fix is withheld when the edit would delete a comment. The diagnostic
remains, because the blank is still unnecessary.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/blankrange/cmd/blankrange
go tool blankrange ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -blankrange ./...
go tool lesiw.io/tools/cmd/fix -blankrange ./...
```
