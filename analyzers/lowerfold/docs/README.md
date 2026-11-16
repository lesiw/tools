# lesiw.io/tools/analyzers/lowerfold

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/lowerfold.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/lowerfold)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of two strings converted to
lower or upper case.

```go
func f(left, right string) bool {
    return strings.ToLower(left) == strings.ToLower(right) // strings.ToLower on both sides; use strings.EqualFold
}
```

The doc for `strings.ToLower` says it "returns s with all Unicode letters
mapped to their lower case", and `strings.ToUpper` does the same for upper
case. Each call builds a new string when the text changes. `strings.EqualFold`
"reports whether s and t, interpreted as UTF-8 strings, are equal under simple
Unicode case-folding" and takes the two originals.

A comparison is reported when both sides call the same function, one of
`ToLower` and `ToUpper` in `strings`, and the operator is `==` or `!=`. An
ordering comparison is left alone, as are a conversion on one side and a pair
that uses different functions. Only a package-qualified call is examined; a call
reached through a dot import or held in a function value is left alone.

Folding and conversion differ for some code points. `strings.ToLower("Σ")` is
`"σ"` and `strings.ToLower("ς")` is `"ς"`, so the comparison is false, while
`strings.EqualFold` compares the two as equal. Go issue 52204 describes the
difference.

The rule is adapted from staticcheck's SA6005.

## Fixes

`strings.ToLower(left) == strings.ToLower(right)` becomes
`strings.EqualFold(left, right)`. A `!=` comparison is negated, as in
`!strings.EqualFold(left, right)`. In a file that imports `strings` under
another name, the fix uses that name, as in `text.EqualFold(left, right)`. Each
operand keeps its source form, so a comment inside one remains. The fix is
withheld where a comment elsewhere in the comparison would be dropped, and
where the comparison spans lines. For the code points above, the rewrite
changes the result, so a caller who needs the exact conversion keeps the
comparison.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/lowerfold/cmd/lowerfold
go tool lowerfold ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -lowerfold ./...
go tool lesiw.io/tools/cmd/fix -lowerfold ./...
```
