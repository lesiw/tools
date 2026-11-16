# lesiw.io/tools/analyzers/structtagdup

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/structtagdup.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/structtagdup)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a struct tag that names the same key twice.

```go
type item struct {
    Name string `json:"name" json:"label"` // struct tag key "json" is repeated
}
```

The reflect package describes a struct tag as "a concatenation of optionally
space-separated `key:"value"` pairs" ([StructTag](https://pkg.go.dev/reflect#StructTag)).
`reflect.StructTag.Lookup` returns "the value associated with key in the tag
string" ([Lookup](https://pkg.go.dev/reflect#StructTag.Lookup)), so it does not
read the later pair.

The rule reports every pair after the first with the same key. Keys are compared
byte for byte after the tag literal is decoded; the values do not matter. The
rule reports the second and third pairs of a tag that names the same key three
times.

Not reported:

- a repeated key of `choice`, `optional-value`, or `default` in a package that
  imports `github.com/jessevdk/go-flags`, where a repeated key holds one value
  per alternative;
- a tag whose literal cannot be decoded, or whose malformed pair comes before
  the repeat, or where the repeat itself is malformed.

The fix removes the later pair and the whitespace before it. The analyzer edits
the tag literal in place, so it does not re-quote a value that holds an escaped
quote or a non-ASCII character.

The rule is adapted from staticcheck's SA5008, which reports a duplicate key
among its other struct tag checks.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/structtagdup/cmd/structtagdup
go tool structtagdup ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -structtagdup ./...
go tool lesiw.io/tools/cmd/fix -structtagdup ./...
```
