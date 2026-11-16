# lesiw.io/tools/analyzers/rangestring

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/rangestring.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/rangestring)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a range over a string that only counts
runes.

```go
func count(s string) int {
    n := 0
    for range s {
        n++
    }
    return n
}
```

The spec says that a range over a string "iterates over the Unicode code
points in the string" (https://go.dev/ref/spec#For_statements). The
`unicode/utf8` package documentation
(https://pkg.go.dev/unicode/utf8#RuneCountInString) says that
`RuneCountInString` "is like `RuneCount` but its input is a string", and
`RuneCount` returns the number of runes, treating an erroneous encoding as one
rune as the range does. A loop that starts a counter at zero and increments it
once per iteration computes the number of runes.

The loop is reported when the counter is declared as `n := 0` immediately
before it, both iteration variables are blank or absent, the body holds only
`n++`, and the range expression is a string that does not refer to the counter.
A loop with a non-blank iteration variable, whose body is anything other than
`n++`, or whose `n := 0` is not the statement immediately before it is left
alone. Only the literal `0` matches; `0x0` and `(0)` are left alone. The
`n := 0` must be in the same statement list as the loop.

The fix writes `n := name.RuneCountInString(s)` in place of the declaration and
the loop, where `name` is the name the file binds to `unicode/utf8`, or `utf8`
when the import is added. A defined string type or a type parameter is
converted, as `name.RuneCountInString(string(w))`. A missing import of
`unicode/utf8` is added as a declaration after the package clause; `cmd/fix`
moves it into the file's import block.

The fix is withheld where the counter is not read outside the loop, where a
comment sits in the text it replaces, where the name the fix writes for
`unicode/utf8` reaches another object at the declaration, where `string` does so
when a conversion is written, and where the file binds `unicode/utf8` to the
dot or blank name. The diagnostic is still reported.

Fixable: the count loop.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/rangestring/cmd/rangestring
go tool rangestring ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -rangestring ./...
go tool lesiw.io/tools/cmd/fix -rangestring ./...
```
