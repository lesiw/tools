# lesiw.io/tools/analyzers/nilchansend

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/nilchansend.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/nilchansend)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a send or receive on a channel that is nil
at that point.

```go
func sendOnNil() {
    var ch chan int
    ch <- 1 // send on nil channel blocks forever
}
```

The specification's Channel types section says "A nil channel is never ready
for communication". The Send statements section says "A send on a nil channel
blocks forever", and the Receive operator section says "Receiving from a nil
channel blocks forever". A range over a nil channel blocks forever too.

The analysis records the channel values it can prove nil at each point. A
local variable declared without a value is nil, as is an explicit nil, a
conversion of nil, a copy of a nil value, and a merge whose incoming values are
all nil. A comparison with nil narrows the value to nil on the nil branch. A
value the analysis proves non-nil is not narrowed. An operation is reported
when the analysis proves the channel nil on every path to it. Each operation
is reported on its own, including one after an operation that blocks forever;
the analysis does not prove reachability. A loop value with an edge that is
not nil is left alone, even when another iteration would use a nil value. A
send and a receive are both reported, as is the receive of a `range` over a
channel.

These values start unknown: a parameter, a package variable, a field, an
element, a call result, a receive, and a type assertion. A parameter and a
local variable can be shown nil at a comparison with nil; this covers a call
result, a receive, and a type assertion once they are assigned to a local
variable. A field, an element, and a package variable are read again at each
use, so a comparison does not narrow them. A variable whose address is taken,
and one captured by a function literal, are left alone, because another
function can write them. A named result whose address is taken, as by a defer,
is left alone for the same reason.

Not reported:

- A send or receive in a select case when the select has more than one case,
  or when it has a default. A nil channel disables that case, and a select with
  every case disabled and no default blocks in the select. A select with a
  single case and no default becomes a plain send or receive and is reported.
  A send or receive in the body of a selected case is reported like any other.
- An operation whose channel is nil on some paths only, unless a preceding
  comparison narrows it to nil.
- A send or receive on a variable captured by a function literal.
- A send or receive in a loop when the loop value is nil on some iterations
  only. The rule follows one value for the loop and reports only when every
  incoming value is nil.
- A call such as `close`, `len`, or `cap` on a nil channel.

The suite's `nilness` pass reports some of the same operations. This rule also
reports an operation after a branch join where every incoming value is nil.
The cmd/vet suite runs both, so an operation that both report gets two
diagnostics.

Report only: a fix would have to choose a channel value, and removing the
operation changes the function's synchronization.

The rule is adapted from staticcheck's SA5011 family.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/nilchansend/cmd/nilchansend
go tool nilchansend ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -nilchansend ./...
```
