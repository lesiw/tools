# lesiw.io/tools/analyzers/cyclo

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/cyclo.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/cyclo)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a function with more branches than fit in
one reading.

The count is cyclomatic complexity: one for the function, one for each `if`,
`for` and `range`, and one for each `&&` or `||`. A function over the limit is
reported at its name.

## Checks

### A function over the limit

The report lands on the function name and names the count and the limit:

```
main.go:12:6: cyclomatic complexity 16 exceeds 15
```

## What the count leaves out

A `switch` or a `select` costs one, and its clauses cost nothing. A jump table,
a state machine and a channel router are dispatch tables, and charging for each
arm makes the number describe the table rather than the function. A `switch`
with forty cases reads as one decision, and a virtual machine is not
expressible without one.

A function literal is measured on its own. A walk defined inside a function is
a walk, and a reader judges it separately from the steps around it.

## What the number is for

A finding means the function has more branches than fit in one reading. The
repair is to give the parts names, which is the same reason a long function
gets split.

The count reads one shape badly: a conjunction of eight terms is one decision
on one line, and it charges eight.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/cyclo/cmd/cyclo
go tool cyclo ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -cyclo ./...
go tool lesiw.io/tools/cmd/fix -cyclo ./...
```

The limit is a flag, `-cyclo.max`, and it defaults to 15.
