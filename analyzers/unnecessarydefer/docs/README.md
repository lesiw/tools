# lesiw.io/tools/analyzers/unnecessarydefer

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/unnecessarydefer.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/unnecessarydefer)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call that can be replaced by a
plain call.

```go
func f() {
	defer close1() // deferred call can be replaced by a plain call
}
```

A call that ends the function runs at the same moment either way. The defer may
be the last statement in the body of an if, a switch, a type switch, a select,
or a block, or the statement a label wraps. It is reported when nothing follows
the construct.

The idea comes from go-critic's unnecessaryDefer.

Only the last defer in a body can be reported. Deferred calls run in reverse
order, so a plain call in place of an earlier one would run before the later
ones.

A defer with a statement after it is left alone, unless that statement is a bare
return, because dropping the keyword would move the call in front of it. A
fallthrough counts as a statement after the defer: the next case's body runs
after it.

A return that lists its results is left alone. Its expressions are evaluated
before the deferred calls run, and a plain call would run before those
expressions. A bare return has no expressions, so a defer before it still runs
last.

A call that returns a value is left alone: a plain call discards a result that
may need reading, which the suite reports as an unchecked error or an unused
result.

A call through a type parameter is left alone, even where the constraint names
one signature, because a type parameter's underlying type is the constraint
interface.

A loop is not looked through, because its deferred calls run after the loop
while a plain call would run at the end of each pass.

A deferred function literal is not reported. A literal called on the spot reads
worse than the defer, and `deferunlambda` reduces the case where the literal
holds one call with no arguments.

Report only: a callee that logs or traces where it was called from, through
`runtime.Caller`, a panic trace, or `testing.T.Helper`, reports a different line
once the defer is gone. The tool cannot tell which callees do that, so the edit
is the author's.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/unnecessarydefer/cmd/unnecessarydefer
go tool unnecessarydefer ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -unnecessarydefer ./...
```
