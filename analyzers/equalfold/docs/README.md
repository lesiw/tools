# lesiw.io/tools/analyzers/equalfold

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/equalfold.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/equalfold)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a comparison of two strings or byte slices
converted to the same case.

```go
func f(a, b string) bool {
    return strings.ToLower(a) == strings.ToLower(b) // strings.ToLower on both sides; use strings.EqualFold if folding is intended
}
```

`bytes.ToLower` "returns a copy of the byte slice s", and `strings.ToLower`
builds a string when a letter changes. A comparison of two such values is two
copies built for one answer.

A comparison is reported only when both sides call the same function, one of
`ToLower` and `ToUpper` in `strings` or `bytes`, and the comparison is written
with `==` or `!=`, or as a `bytes.Equal` call. A conversion on one side only is
a different question: `strings.ToLower(a) == b` asks whether `a` lowers to
exactly `b`, and this rule compares two converted values. A switch on a converted value is left alone,
even where a case converts the same way. Only a package-qualified call is
examined, so a conversion reached through a dot import or a function value is
not.

Folding and conversion differ for some code points. `strings.EqualFold`'s doc
says it compares "under simple Unicode case-folding, which is a more general
form of case-insensitivity", and Go issue 52204 is about the difference.
`strings.ToLower("Σ")` is `"σ"` and `strings.ToLower("ς")` is `"ς"`, so
lowering separates a sigma from a final sigma where folding compares them
equal. `strings.ToLower("İ")` is `"i"` and `strings.ToUpper("ı")` is `"I"`, so
conversion joins `İ` to `i` and `ı` to `I` where folding keeps both pairs
apart.

Where folding was meant, the copies are not needed: `strings.EqualFold` takes
the two originals and allocates nothing, and `bytes.EqualFold` compares the
byte slices without copying them.

A hash table keyed by a converted value has to keep the conversion:
`hash/maphash`'s `Hasher` doc writes it that way, and says folding is not
consistent with `ToLower` for all values.

The rule is adapted from go-critic's equalFold, which reports a comparison with
only one side converted too.

Report only: the swap changes the result for the code points where folding and
conversion differ.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/equalfold/cmd/equalfold
go tool equalfold ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -equalfold ./...
```
