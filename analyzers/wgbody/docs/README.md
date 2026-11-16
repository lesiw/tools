# lesiw.io/tools/analyzers/wgbody

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/wgbody.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/wgbody)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `Done` call or a `panic` inside a
`sync.WaitGroup.Go` body.

```go
func start(wg *sync.WaitGroup) {
    wg.Go(func() {
        defer wg.Done() // removes the task Go added
    })
}
```

[`WaitGroup.Go`](https://pkg.go.dev/sync#WaitGroup.Go)'s documentation says "Go
calls f in a new goroutine and adds that task to the WaitGroup. When f returns,
the task is removed from the WaitGroup." A `Done` call inside the body removes
the task `Go` added. If it runs before the body returns, a `Wait` can return
while the body still runs. `Go` removes the task again when the body returns,
and the count can become negative. The same documentation says "The function f
must not panic."

`WaitGroup.Go` was added in Go 1.25.

A `Done` on the same `WaitGroup` is reported unless an earlier, non-deferred,
non-goroutine `Add` on that group is in the same block or clause, or an
enclosing block. The `Add` must be in the declared function that contains the
`Go` call, or in the outermost function literal when there is no declaration. A
constant `Add` delta covers that many `Done` calls, and a non-constant delta
covers any number; a `Done` written once counts once even if a loop runs it
more than once. A `Done` outside the `Go` body and a negative `Add` do not
reduce an `Add`'s capacity. An `Add` in a branch that does not enclose the
`Done` does not cover it. An `Add` on another `WaitGroup`
does not cover it, and neither does an `Add` with a constant delta of zero or
less. A `Done` on another `WaitGroup` is not reported.

A `Done` or `Add` is recognized only when its receiver is the same group value,
allowing parentheses, a dereference, or an address; a second name for the group
is not recognized. A method expression, an embedded `WaitGroup`, and a type
alias are recognized.

A `Done` and a `panic` are read in the statements of the body, in a direct call
made by a `go` statement, and in a function literal that is the callee of a
call or defer there. A literal passed to another function, including another
`Go` call, assigned to a variable, or started with `go` is not followed. The
builtin `panic` and the `log.Panic` variants are read. A `panic` after a
deferred function literal that calls `recover` in an expression statement, an
assignment, or an `if`, `switch`, or type-switch init before the panic is not
reported; a `recover` under a branch, after the panic, or inside a nested
literal does not count. A
deliberate panic can be marked with the suite's `//ignore:wgbody` directive.

Only a function literal passed to `Go` is read. A function name, a method
value, or any other value passed to `Go` is not read.

Report only: deleting a `Done` can leave a count that an `Add` in another
function raised, and deleting a `panic` changes what the body does when it
panics.

The rule is inspired by revive's `forbidden-call-in-wg-go`.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/wgbody/cmd/wgbody
go tool wgbody ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -wgbody ./...
```
