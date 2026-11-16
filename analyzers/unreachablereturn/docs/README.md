# lesiw.io/tools/analyzers/unreachablereturn

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unreachablereturn.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unreachablereturn)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a return statement after a panic or an
endless loop in the same block.

An endless loop is a `for` statement that cannot complete normally.

```go
func f() {
    panic("some message")
    return // return is unreachable after a panic
}
```

The spec treats a call to panic as terminating, and a `for` statement with no
condition, no break referring to it, and no range clause as terminating
([Terminating statements](https://go.dev/ref/spec#Terminating_statements)).
Neither falls through, so a return after one cannot run unless a goto
reaches it.

```go
func wait() {
    for {
    }
    return // return is unreachable after a loop that cannot complete normally
}
```

A call through a variable or function named `panic` is not a call to the
builtin. A panic in a deferred call or a goroutine does not stop the function
at that statement. The rule follows the spec's terminating form of a `for`
statement and does not evaluate a condition, so a loop with a condition is
left alone.

A `for` statement with no condition is endless to the rule unless a break
targets the loop. A break to an enclosing statement skips the rest of the
block after the inner loop, so a return there is reported.

A goto target is treated as reachable, even when the goto itself cannot run. A
return after a loop that cannot complete normally, before the label of a
forward goto out of the loop, is reported unless another goto target comes
between them. A return after the label is reported when a panic or an endless
loop comes between the label and the return.

The rule checks block statements and the bodies of case and comm clauses.

The rule is inspired by the x/tools `unreachable` pass, which reports
unreachable code. The default suite runs both analyzers, so most returns
reported here also get an `unreachable code` diagnostic. A parenthesized panic
is one the x/tools pass does not recognize.

Report only: removing the return would drop any comment on it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unreachablereturn/cmd/unreachablereturn
go tool unreachablereturn ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unreachablereturn ./...
```
