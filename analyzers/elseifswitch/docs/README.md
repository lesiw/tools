# lesiw.io/tools/analyzers/elseifswitch

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/elseifswitch.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/elseifswitch)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports an if-else chain that compares one
expression with `==` on every branch.

```go
func pick(kind string) int {
    if kind == "alpha" { // if-else chain on kind; write a switch
        return 1
    } else if kind == "beta" {
        return 2
    }
    return 0
}
```

Effective Go's section on switch gives the reason: "It's therefore
possible—and idiomatic—to write an if-else-if-else chain as a switch"
(https://go.dev/doc/effective_go#switch). Each branch of the chain becomes one
case clause, and a branch whose condition joins comparisons with `||` becomes a
clause that lists the values it compares, since the same section notes that
cases "can be presented in comma-separated lists":

```go
func pick(kind string) int {
    switch kind {
    case "alpha":
        return 1
    case "beta":
        return 2
    }
    return 0
}
```

A switch reads its expression once, where a chain may read it several times, so
the expression every comparison reads must give the same value on a second
read. The rule accepts a call only when it is a conversion or a builtin that
only reads its arguments, and leaves out a length read on a channel because
another goroutine can change it.

A chain needs two ifs: a single if and an else is a choice between two things,
however many values its condition lists.

A switch compares each case value with its expression by equality, so only an
`==` comparison has a case form: a chain that compares with another operator,
or whose branches compare different expressions, is left alone. So are:

- a chain whose ifs bind an init statement;
- a chain whose expression is untyped, which a switch types on its own: the
  spec converts an untyped constant to its default type and does not allow
  `nil` as a switch expression
  (https://go.dev/ref/spec#Expression_switches);
- a chain that names one value in two branches, which a switch rejects and
  `dupelseif` reports;
- a branch that holds a bare `break` outside a nested loop, switch, or select.
  The switch would take the break as its own, and the loop or switch around the
  if would not be left. A labeled break names its target, and a switch is not a
  `continue` target, so neither changes.

The rule reads a chain from its first if, so a chain that begins at an `else if`
is left alone. One that is alone in an else block is left to `elseif`, which
folds the block into `else if`.

The rule is adapted from staticcheck's `QF1003`, which reports an if-else-if
chain that compares one value with `==`.

Fixable: the first if becomes the switch header, each branch becomes a case
clause, and an else block becomes `default`. A clause that does not fit on one
line is wrapped one value per line. The fix is withheld, and the chain is still
reported, when:

- a comment falls in the text it deletes;
- the case values are a contiguous run of integer literals in an order
  `caseorder` rejects, and no fix rearranges a clause;
- the if, a body's first statement, or a body's closing brace does not begin
  its own line, which leaves the clause with nothing to start. `gofmt` writes
  all three at a line start, so this holds for the source as the driver read
  it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/elseifswitch/cmd/elseifswitch
go tool elseifswitch ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -elseifswitch ./...
go tool lesiw.io/tools/cmd/fix -elseifswitch ./...
```
