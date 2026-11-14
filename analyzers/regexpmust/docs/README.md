# lesiw.io/tools/analyzers/regexpmust

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/regexpmust.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/regexpmust)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `regexp.Compile` of a constant pattern
whose error is always nil, so any check for it is dead code.

```go
func must() *regexp.Regexp {
	re, _ := regexp.Compile(`^a+$`) // the error of a constant pattern is always nil; use MustCompile
	return re
}
```

`CompilePOSIX` takes `MustCompilePOSIX`, which keeps the pattern in POSIX ERE.

Only a constant argument to a package-qualified call is checked: a pattern held
in a variable or a parameter is left alone, and so is a call through a dot
import or a function value.

A pattern that does not compile is a different fault, reported by `regexpconst`:
moving it to `MustCompile` would turn the returned error into a panic without
fixing the pattern.

The rule is adapted from go-critic's regexpMust.

Fixable, where the pattern is bound and the second result is blank. A report
where the caller reads the error, both results are discarded, or a comment sits
between the targets has no fix.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/regexpmust/cmd/regexpmust
go tool regexpmust ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -regexpmust ./...
go tool lesiw.io/tools/cmd/fix -regexpmust ./...
```
