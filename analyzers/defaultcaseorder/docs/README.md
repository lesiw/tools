# lesiw.io/tools/analyzers/defaultcaseorder

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/defaultcaseorder.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/defaultcaseorder)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a default clause in the middle of a
switch, a type switch, or a select.

```go
func f(n int) int {
    switch n {
    case 1:
        return 1
    default: // default clause is in the middle
        return 0
    case 2:
        return 2
    }
}
```

A default clause belongs at the top of the clause list or at the end of it. The
clause that runs is the same either way, so the position is presentational: this
is a house rule, and the suite writes the default clause where a reader looks
for it.

A default clause that a case falls through into is not reported: moving it
changes where the jump lands. One that ends in a fallthrough is not reported
either: at the end of the list it would have no clause to enter. A labeled
fallthrough counts.

The fix writes the clause after the last one. It carries the comment groups
written above the clause at its own indentation, and the blank lines above them.
A group indented deeper than the clause stays with the clause above it.

`caseorder` leaves a switch that holds a middle default alone. The move does not
touch the other clauses, so `caseorder` reports them once the default no longer
splits the run.

A three-way comparison written as `>`, `=`, `<` puts its equality clause in the
middle, and the fix overwrites that order: the clause runs `>`, `<`, `=`. Every
position the rule allows reorders the run, so `//ignore` is how a switch keeps
`>`, `=`, `<`.

## When the fix is withheld

A clause is reported without a fix in these cases:

- The clause does not start its line.
- The clause below it does not start its line.
- The closing brace does not start its line.
- A comment group above it is indented shallower than it, so the group belongs
  to no clause and the move would leave it over the clause below.
- A comment group above the closing brace is indented no deeper than the last
  clause: the clause would be written below it.
- A switch in the clause's body asks for a fix in the same run. The inner switch
  waits for the next run, since fixes that overlap cannot both apply.
- The file changed size since it was parsed, and the offsets the move splices
  would point into other text.

The rule is adapted from go-critic's `defaultCaseOrder`, which reports the same
shape in an expression switch and offers no fix.

Fixable.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/defaultcaseorder/cmd/defaultcaseorder
go tool defaultcaseorder ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -defaultcaseorder ./...
go tool lesiw.io/tools/cmd/fix -defaultcaseorder ./...
```
