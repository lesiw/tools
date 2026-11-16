# lesiw.io/tools/analyzers/deferinfinite

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferinfinite.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferinfinite)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call in a loop that cannot end.

```go
func f() {
    for {
        defer release() // deferred call in this loop never runs
    }
}
```

The For statements section of the spec says a for statement with no condition
is equivalent to one whose condition is true
(https://go.dev/ref/spec#For_statements). A condition that is a constant true
has the same effect. The Defer statements section says a deferred call runs
when the surrounding function returns, including when it panics
(https://go.dev/ref/spec#Defer_statements). A call in a loop that cannot end
does not run before the function returns, and each pass adds another call to
the function's list.

The loop is checked when its condition is absent or a constant true. It is not
reported when the body has a return, a break, a goto, or a labeled continue
that leaves it. A direct `panic` or `runtime.Goexit` call in the loop body or
post statement keeps the deferred call from being reported, even when the
branch holding it cannot run. This includes a call in a function literal
invoked where it is written, but not a call inside a deferred or started
function literal. `runtime.Goexit` terminates the goroutine after it runs the
deferred calls (https://pkg.go.dev/runtime#Goexit). Other calls are assumed to
return.

An unlabeled break leaves the nearest loop, switch, or select. An unlabeled
continue starts the next pass of the nearest loop. A labeled break leaves the
statement its label names, and a labeled continue starts the next pass of the
loop its label names. A break or continue to a label outside the current loop
leaves it, and one to a label inside a nested statement stays.

A goto to a label inside the loop stays in the loop. A goto to a label before
the loop stays when no return, branch that leaves, goto, or direct `panic` or
`runtime.Goexit` call lies between the label and the loop. A goto to a label
after the loop counts as an exit, even when another goto leads back to the
loop.

A defer inside an inner loop is reported when any loop around it cannot end,
even if the inner loop can. The diagnostic is placed at the defer.

A function literal is a separate function. A defer directly in one, outside a
loop, runs when the literal returns, so it is not reported. A loop inside a
literal is checked on its own.

A range loop is not checked, unless an endless for encloses it.

Each defer is reported once, even when one endless loop encloses another.

A return after a nested loop that cannot end still counts as an exit, so a
defer in that body is not reported.

No fix is offered: moving the call out of the loop or running it plainly
changes when it runs, and either may be the repair the author wants.

The rule is adapted from staticcheck's SA5003
(https://staticcheck.dev/docs/checks/#SA5003).

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferinfinite/cmd/deferinfinite
go tool deferinfinite ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferinfinite ./...
```
