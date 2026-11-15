# lesiw.io/tools/analyzers/caseorder

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/caseorder.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/caseorder)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a switch whose case values are a contiguous
run in neither ascending nor descending order.

```go
func f(n int) {
    switch n { // case values are in neither ascending nor descending order
    case 3:
    case 1:
    case 2:
    }
}
```

A reader has no place to start in a switch that runs 3, 1, 2, so they read
every clause. Literal cases with no fallthrough behave the same in any order,
so the order is presentational: this is a house rule, and the suite writes a
run of values in one direction.

Every case expression must be an integer literal, written without a sign, with
or without parentheses, and fitting in an `int64`; a switch with any other case
expression is left alone. The tag's type does not decide the report: `case 3` in
a switch over a `float64` is reported.

A named constant is left alone: the declaration suggests the order, and the
number behind it is an implementation detail.

A character literal is left alone: the value it stands for is not written in the
source.

A switch with a clause that lists several values is left alone: the clause
covers a set of values at once, so it has no single place in the run.

A switch whose values are not a contiguous run is left alone: a sparse set
usually groups values by meaning rather than by value.

A switch whose default clause is in the middle is left alone: the default
splits the clauses into two runs, so there is no one run to read.
`defaultcaseorder` moves the clause last.

A switch with a clause that ends in a fallthrough is left alone: the clause
order is part of the program.

The name comes from go-critic's `caseOrder`, which reports a type switch clause
that can never run.

Report only: a clause's leading comment is outside the clause, so a rewrite
would drop it or guess which clause it belongs to.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/caseorder/cmd/caseorder
go tool caseorder ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -caseorder ./...
```
