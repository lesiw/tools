# lesiw.io/tools/analyzers/niltimer

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/niltimer.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/niltimer)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a Timer or Ticker method call on a nil
pointer.

```go
func stop() {
    var t *time.Timer
    t.Stop() // call to Stop on a nil Timer
}
```

The Timer documentation says a Timer must be created with `NewTimer` or
`AfterFunc`. Each Timer and Ticker method reads a field of the receiver:
`if !t.initTimer` in `Timer.Stop` and `Timer.Reset`, and `if !t.initTicker` in
`Ticker.Stop` and `Ticker.Reset` (https://go.dev/src/time/sleep.go,
https://go.dev/src/time/tick.go). A nil receiver panics during the call.
`TestIssue5745` in the time package calls Stop on a nil Timer to exercise the
panic (https://go.dev/src/time/sleep_test.go).

The rule reports a call when the receiver's value is nil. A variable copied
from another is still nil, and so is a merge of nil values. A variable
assigned a timer on one branch before the call is not reported; a loop
backedge that assigns a timer counts as such a branch. An assignment of nil
leaves the receiver nil. A named result starts nil. Without a defer in the
function, a call on one is reported when no assignment reaches it; a defer
moves the result to memory, which the rule does not follow. A call inside a
nil test is still reported, even when the condition is always false.

A nil constant, a conversion of nil, and a merge of nil values are recognized.
A load from memory is not. Not reported, and why:

- a receiver the rule does not track: a field, a package variable, a
  collection element, or a channel receive;
- an incoming parameter, a call result, or a type assertion, which the rule
  does not resolve;
- a receiver whose address is taken, or that a function literal captures,
  since code the rule does not follow may set it first;
- a call through an interface value, whose implementation is not known until
  run time;
- a stored method value, since the rule does not follow the closure that
  captures the receiver; an immediate method value is reported, since it
  lowers to a direct call;
- a method expression loaded from a package variable or field; a method
  expression called directly or through a local value is reported;
- a call in a package-level variable initializer, including a function
  literal there, since the package initialization function is not analyzed;
- the zero Timer or Ticker value, which is not nil and so is outside this
  rule; some of its methods panic as uninitialized, which is a different fault.

Report only: two repairs are plausible, deleting the call or guarding it, and
the choice belongs to the author.

The rule is adapted from staticcheck's SA1019 family.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/niltimer/cmd/niltimer
go tool niltimer ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -niltimer ./...
```
