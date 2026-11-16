# lesiw.io/tools/analyzers/regexpraw

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/regexpraw.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/regexpraw)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a regexp pattern written as an interpreted
string that doubles every backslash and holds at least one.

```go
regexp.MustCompile("\\d+") // pattern literal has doubled backslashes; use a raw string literal
```

The spec's [String literals](https://go.dev/ref/spec#String_literals) say a raw
string keeps a backslash as written and drops a carriage return, so
`` `\d+` `` and `"\\d+"` hold the same pattern.

The package-level functions in `regexp` that take a pattern are checked:
`Compile`, `CompilePOSIX`, `Match`, `MatchReader`, `MatchString`,
`MustCompile` and `MustCompilePOSIX`. The callee comes from the type checker,
so an aliased or a dot import is covered. A method such as `re.MatchString`
takes the text to match, so it is left alone. A call through a function value
is not examined, and a pattern held in a constant, a variable, or a parameter
is left alone. A pattern built by concatenation or conversion is left alone
too.

A literal is reported when it is an interpreted string that doubles every
backslash and holds at least one. A raw string literal needs no rewrite. An
escape other than a doubled backslash fails the condition, so it is not
reported. Two more literals are left alone:

- one holding a backtick, which a raw string cannot contain;
- one whose value holds a carriage return, which a raw string literal discards.

The fix replaces the literal with the raw form.

The rule is inspired by staticcheck's `S1007`, which checks `regexp.Compile`
and `regexp.MustCompile`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/regexpraw/cmd/regexpraw
go tool regexpraw ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -regexpraw ./...
go tool lesiw.io/tools/cmd/fix -regexpraw ./...
```
