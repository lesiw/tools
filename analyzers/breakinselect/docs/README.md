# lesiw.io/tools/analyzers/breakinselect

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/breakinselect.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/breakinselect)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a break at the end of a select case in a
loop when it leaves only the select.

```go
func f(ch <-chan int) {
    for {
        select {
        case <-ch:
            break // break at the end of a select case has no effect
        }
    }
}
```

A break inside a select leaves the select. A break at the end of a case that
leaves only the select has no effect, because the select ends either way. A
labeled break whose label names a loop, switch, or another select is not
reported. Execution continues after the select, and a break meant to leave the
loop does not leave it.

The analyzer checks the end of a case through a trailing if, else, block, or
labeled statement, and reports each break it finds. A case whose trailing if
and else both end in break gets two reports.

A labeled break at the end of a case is reported when its label names the
select being checked, since that break leaves only the select. A label on a
loop, switch, or other select that contains the case makes the break leave that
statement, so it is silent.

Silent, deliberately:

- A break that a statement in the case follows, because the break skips that
  statement and has an effect.
- A select outside a loop, because there is no enclosing loop for the break to
  leave.
- A select whose enclosing loop lies outside the function literal that
  contains the select, because a break in the literal cannot leave that loop.
  A loop inside the literal is checked.

Out of scope: a break at the end of a switch case inside a loop. The analyzer
checks only the statements that end a case, through trailing if, else, block,
and labeled statements, so a break inside a nested for, switch, or select is
not checked again. A nested select is checked on its own, and a break there
that names an outer select is outside this analyzer's scope.

Report only: the repair is a label on the loop or a return, and the tool cannot
determine which one the author meant.

The rule is adapted from staticcheck's SA4011, which also covers a switch case
inside a loop.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/breakinselect/cmd/breakinselect
go tool breakinselect ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -breakinselect ./...
```
