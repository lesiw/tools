# lesiw.io/tools/analyzers/wgogoroutine

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/wgogoroutine.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/wgogoroutine)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a `sync.WaitGroup` count raised from inside
a goroutine instead of before it starts.

```go
func serve(wg *sync.WaitGroup) {
    go func() {
        wg.Add(1) // Add raised from inside a goroutine instead of before it starts
        defer wg.Done()
        work()
    }()
    wg.Wait()
}
```

A positive delta on a counter of zero raises it, and `Add`'s documentation
requires that call to happen before a `Wait`. Nothing in the literal orders the
call before the `Wait` that follows it, so the `Wait` can observe a zero counter
and return before the goroutine has counted itself. An `Add` that raises the
count from zero while a `Wait` is blocked panics. `Go` raises the same count,
and its documentation says "If the WaitGroup is empty, Go must happen before a
Wait".

Where an `Add` before the `go` statement already counted the goroutine, and
nothing lowers the count the call raises, that count stays outstanding and
`Wait` does not return.

A `defer wg.Done()`, or a `defer wg.Add(n)` with a negative constant `n`, is
evidence that the goroutine holds a count of its own until it returns, which is
how a worker goroutine that starts further goroutines is written. `Add`'s
documentation permits a positive delta while the counter is above zero, so a
call after one is left alone. The evidence is the last lower the literal writes
before the call, and only when that lower is deferred: a plain `Done` written
closer to the call may have released the count the deferred lower was holding,
so the call may raise from zero. Where the caller did not count the goroutine,
the exemption gives up the report, and the `Add` can still race with a `Wait`
the caller makes.

A balanced fan-out whose lower is written plainly at the end of the goroutine is
therefore reported: the literal does not show that the count was raised before
it started. Deferring the lower stops the report, and it keeps the release on an
early return too.

A group the literal creates is never reported. Nothing could have waited on it
before the goroutine that made it started, and a `Wait` the literal makes later
is answered by the rule above. That covers a composite literal, a value from
`new`, and a variable the literal declares without a value.

A group a call returns is reported, since the call may hand out a group from
elsewhere. So are a name the literal declares from a group from outside, as in
`alias := wg`, and a group it reaches through a slice or a map. A group the
literal takes as a parameter is reported the same way as any other, since the
caller may also wait on it.

A `Wait` the literal makes on the same group after the call, in the literal's
own block or under a branch, answers the report, though the caller may still
wait on that group. The whole body of the literal is read, stopping where a
nested literal begins.

A literal handed to a function is out of reach. That covers `WaitGroup.Go`,
which raises the count before it starts the task and so leaves the counter above
zero for the whole literal. A literal the goroutine calls in place is out of
reach too, in a `defer` or otherwise.

A `go` statement that starts a named function is out of reach, and so is one
whose call is the `Add` itself, as in `go wg.Add(1)`: the rule reads a goroutine
that is a function literal. An `Add` reached through a method value is out of
reach as well, because the group is bound where the value is made and the call
names no group.

The methods have to be the ones `sync.WaitGroup` declares. Each of them has a
pointer receiver, so a value, a pointer, and a promoted method are recognized
the same way; a local type with methods of those names does not match.

A receiver counts whichever way it is written, so `wg`, `*wg`, and `&wg` name
one group.

A call whose delta is a constant of zero or less does not raise the count and is
not reported. A delta that is not constant may raise it, so the call is
reported.

The rule is adapted from the x/tools vet pass `waitgroup` and staticcheck's
`SA2000`, both of which read an `Add` written as the first statement of the
goroutine's literal. This rule reads every statement of the literal.

Report only: hoisting the count above the statement that starts the goroutine
changes what the group counts whenever the call is conditional or sits in a
loop, and where the group is a parameter of the literal, the hoist would have to
write the argument's name at the call site. `WaitGroup.Go`, added in Go 1.25,
raises the count before it starts the task and needs no hoist, and the suite's
`waitgroupgo` pass writes that form where an `Add` already sits before the `go`
statement it pairs with.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/wgogoroutine/cmd/wgogoroutine
go tool wgogoroutine ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -wgogoroutine ./...
```
