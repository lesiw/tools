# lesiw.io/tools/analyzers/resetvalue

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/resetvalue.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/resetvalue)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a read of the value returned by
`(*time.Timer).Reset` that decides whether to drain the timer's channel.

```go
func restart(t *time.Timer) {
    if !t.Reset(time.Second) {
        <-t.C
    }
}
```

`Timer.Reset`'s documentation says it "returns true if the timer had been
active, false if the timer had expired or been stopped"
(<https://pkg.go.dev/time#Timer.Reset>). Before Go 1.23 the channel was
buffered, so an expired timer's value could still be received, and the result
was read to decide whether to drain it. Draining after `Reset` races with the
new timer, which can send before the receive runs.

Since Go 1.23 a chan-based timer created with `NewTimer` has an unbuffered
channel; the same documentation says "any receive from t.C after Reset has
returned is guaranteed not to receive a time value corresponding to the
previous timer settings". A drain after `Reset` therefore receives the new
timer's value or blocks, so reading the result to choose the branch is wrong
at every Go version.

The rule matches the branch that runs when `Reset` returns false, when that
branch receives from the timer's channel. The condition is a `Reset` call, with
or without a negation, or a local assigned by `Reset` and tested by the next
statement, by the statement before a labeled `if`, or by the `if`'s init. A
branch that runs when `Reset` returns true is a wait, and it is left alone. The
rule follows assignments lexically: an assignment to the timer, or to a value
it is selected from, between the `Reset` and the receive stops the search, as
does a `Reset` or `Stop` call on the timer. Not reported, and why:

- a bare call or an assignment to the blank identifier, whose result is
  discarded;
- a result used for another purpose, such as a log or a branch that receives
  from no channel;
- a receive from another channel, including another timer's;
- a receive inside a function literal, which runs at another time;
- a receive after a write to the timer, or to a value it is selected from,
  earlier in the branch;
- a receive in an else-if body whose own condition is not a `Reset` call; a
  `Reset`-conditioned else-if is reported without a fix;
- a condition that combines the result with another expression;
- a switch or loop header;
- a method value, which the rule does not follow to the call that runs it;
- a method expression, whose receiver is the first argument;
- a local tested by an else-if without an init, which the rule does not follow;
- a local the if init mentions, which stops the search;
- a call in a test file, because a test may assert the documented result;
- a `Reset` method on another type, even one with the same signature and a `C`
  field.

## Fixes

At Go 1.27 and later, or when the file names no version, an `if` whose
condition is the negated `Reset` call, whose body is only the receive, and
which has no init or `else` and is not an else-if is replaced by the `Reset`
call. The edit is withheld when the `if` contains a comment, when the body
holds another statement, and when the `if` has an init or an `else`.

Go 1.23 through 1.26 accept a `godebug` line of `asynctimerchan=1`, which
restores the buffered channel (<https://go.dev/doc/godebug#go-123>), and Go
1.27 removed the setting (<https://go.dev/doc/godebug#go-127>), so the rule
offers the fix only from Go 1.27. Below Go 1.23 the safe form is to stop and
drain before `Reset`, and the rule reports without a fix.

The rule is adapted from staticcheck's SA1025, which reports a `Reset` result
that gates a receive from any channel of type `<-chan time.Time`. This rule
requires the receive and the `Reset` call to name the same timer and can
delete the branch.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/resetvalue/cmd/resetvalue
go tool resetvalue ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -resetvalue ./...
go tool lesiw.io/tools/cmd/fix -resetvalue ./...
```
