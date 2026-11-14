# lesiw.io/tools/analyzers/regexpconst

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/regexpconst.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/regexpconst)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a constant regexp pattern that does not
compile.

```go
var re = regexp.MustCompile(`a(`) // pattern does not compile: missing closing )
```

The pattern never changes, so the call fails the same way every time:
`MustCompile` and `MustCompilePOSIX` panic, and every other function returns the
error.

The functions in `regexp` that take a pattern are checked: `Compile`,
`CompilePOSIX`, `MustCompile`, `MustCompilePOSIX`, `Match`, `MatchReader` and
`MatchString`. The two POSIX functions parse the pattern as POSIX ERE, so `\d`,
which the Perl syntax accepts, does not compile there. Only package-qualified
calls are matched: a call through a dot import or a function value is not. A
pattern held in a variable or a parameter is left alone, because the analyzer
cannot know what it holds at run time. A pattern that is deliberately invalid,
as in an example that prints the parse error, is reported too.

The rule is adapted from staticcheck's `SA1000`, which does not check the POSIX
functions.

Report only.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/regexpconst/cmd/regexpconst
go tool regexpconst ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -regexpconst ./...
```
