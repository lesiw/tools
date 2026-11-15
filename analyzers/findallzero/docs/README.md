# lesiw.io/tools/analyzers/findallzero

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/findallzero.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/findallzero)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `FindAll` call on a `regexp.Regexp` with
a count of zero.

```go
func words(s string) []string {
    return regexp.MustCompile(`\w+`).FindAllString(s, 0) // a count of zero always returns no matches
}
```

A FindAll method returns at most `n` matches when `n >= 0`, and every match
otherwise, so a count of zero never returns a match. This covers all eight
`FindAll` methods of `regexp.Regexp`.

Only a constant count is checked. A count held in a variable is left alone, and
so is a call through a method value or an interface, where the analyzer cannot
see which method is reached.

The rule is like staticcheck's `SA1010`.

Report only: the author may have meant every match (`-1`), or only whether
something matched (`MatchString`).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/findallzero/cmd/findallzero
go tool findallzero ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -findallzero ./...
```
