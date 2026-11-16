# lesiw.io/tools/analyzers/selectempty

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/selectempty.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/selectempty)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a select statement with no cases.

```go
func wait() {
    select {} // select with no cases blocks forever
}
```

The select statements section of the Go spec says a select with no default
case "blocks until at least one of the communications can proceed", and its
example marks `select {}` with `block forever`.

A select with no cases is legal, and a goroutine can block on one on purpose.
It is still easy to mistake for a select whose cases were dropped, so the
rule reports every empty select.

The rule reports the select in a function or method, a function literal, or a
goroutine's function literal. Comments inside the braces do not count as
cases, so a commented empty select is reported too.

A select with a communication case or a default clause is left alone, even
when the case body is empty. A select that waits on a nil channel has cases as
well, and is left alone.

Report only: writing the missing cases changes what the code waits for, and
the tool cannot tell which cases the author meant.

The rule is adapted from staticcheck's SA5004, which reports an empty default
branch in a for-select loop.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/selectempty/cmd/selectempty
go tool selectempty ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -selectempty ./...
```
