# lesiw.io/tools/analyzers/deferinloop

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferinloop.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferinloop)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call in a loop body.

```go
for _, name := range names {
	h := open(name)
	defer h.close() // deferred calls in a loop accumulate until the function returns
}
```

The specification's Defer statements section says deferred calls run "immediately
before the surrounding function returns, in the reverse order they were
deferred". The handle the loop opened is still open when the next iteration
starts, and each iteration leaves one more call for the function.

The analyzer asks one question of the control-flow graph: can control leave the
block that holds the defer and come back to it through a loop? It leaves alone
the flow that cannot:

- a body whose later paths all `return`, `panic`, `break` or `goto` out, so
  control never returns to the defer;
- a call to a function that never returns, which ends the block the call is in;
- a `for` statement with no condition that nothing breaks out of;
- a `switch` or a `select` after the defer that must run a clause and whose
  clauses all leave.

Control also does not come back to a defer in a function literal in the loop,
which returns every time it is called. A cycle that a `goto` makes with no loop
in it is not read, and neither is a test file: a test releases each case's
cleanup through a defer, so that `t.Fatal`, which ends the test from anywhere,
still runs it.

A `continue` arrives at the head of the loop wherever it sits, so a defer above
one is reported. Trip counts are not read, so a loop whose bound runs it once is
reported. A loop written inside a function literal is examined like any other
loop.

The rule is like go-critic's deferInLoop, narrowed to the loop bodies where the
deferral accumulates.

Report only. A call moved out of the defer stops running on the paths that
leave the function, so the edit is the author's: either the release belongs at
the end of each iteration, or the loop holds it on purpose and the iteration
body belongs in a function whose return releases it.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferinloop/cmd/deferinloop
go tool deferinloop ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferinloop ./...
```
