# lesiw.io/tools/analyzers/urlconst

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/urlconst.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/urlconst)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a constant URL that `url.Parse` rejects.

```go
url.Parse("http://[::1") // URL does not parse: missing ']' in host
```

`net/url.Parse` parses a raw URL into a structure. Its documentation says the
URL may be relative, such as a path without a host, or absolute, such as one
starting with a scheme. `url.Parse` returns the same error for a rejected
constant wherever the call runs.

Only a constant argument to a direct `url.Parse` call is checked. A constant
expression is evaluated, so a named constant and a concatenation of literals
are checked as well as a literal. A URL held in a variable or a parameter is
left alone, because its value is not known until the call runs, and so is a
call through a function value. A `Parse` method on a `URL` is a different
function and is not checked. A deliberately malformed URL, as in an example
that prints the error, is reported too.

`url.ParseRequestURI` is not checked. It interprets its argument as an HTTP
request target, which is a different shape from the raw URL that `Parse`
takes.

The rule is inspired by staticcheck's `SA1007`.

Report only: a malformed URL has no mechanical repair, and only the author
knows which URL was meant.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/urlconst/cmd/urlconst
go tool urlconst ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -urlconst ./...
```
