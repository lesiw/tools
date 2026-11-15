# lesiw.io/tools/analyzers/initclause

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/initclause.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/initclause)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)
An `analysis.Analyzer` that reports an if or switch init clause holding a
statement that declares nothing.

```go
func reload(c *cache) bool {
    if c.fetch(); c.err != nil { // if init clause declares nothing
        return false
    }
    return true
}
```

The spec admits any simple statement in the clause. Effective Go describes the
common one: "Since `if` and `switch` accept an initialization statement, it's
common to see one used to set up a local variable." A call, a send, or an
increment sets up no variable, so the clause reads as a declaration and is
something else.

A short variable declaration parses as an `AssignStmt` with `token.DEFINE`, so
one type assertion exempts declarations and assignments alike.

Report-only: hoisting a statement out of the clause loses the comment above it,
an `else if` clause has no statement position to hoist into, and a labelled
`switch` stops compiling once a statement is inserted under the label, because
the label takes the hoisted statement.

The rule is like go-critic's `initClause`, which tags it Style, Opinionated and
Experimental.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/initclause/cmd/initclause
go tool initclause ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -initclause ./...
go tool lesiw.io/tools/cmd/fix -initclause ./...
```
