# lesiw.io/tools/analyzers/deferchain

[![Go Reference](https://pkg.go.dev/badge/lesiw.io/tools/analyzers/deferchain.svg)](https://pkg.go.dev/lesiw.io/tools/analyzers/deferchain)
[![CI](https://github.com/lesiw/tools/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/lesiw/tools/actions/workflows/main.yml)
[![Release](https://img.shields.io/github/v/tag/lesiw/tools?sort=semver&label=release)](https://github.com/lesiw/tools/tags)
[![Go Version](https://img.shields.io/github/go-mod/go-version/lesiw/tools)](../../../go.mod)
[![Discord](https://img.shields.io/discord/1145827224516300971?logo=discord&logoColor=white&color=5865F2&label=discord)](https://lesiw.dev/discord)
[![License](https://img.shields.io/github/license/lesiw/tools)](../../../LICENSE)

An `analysis.Analyzer` that reports a deferred call that returns a function.

```go
func run(cleanup func() func()) {
    defer cleanup() // deferred call returns a function that is not called
}
```

The spec's Defer statements section says "Each time a "defer" statement
executes, the function value and parameters to the call are evaluated as usual
and saved anew but the actual function is not invoked."
(https://go.dev/ref/spec#Defer_statements). In this defer, `cleanup` runs at
the surrounding function's return, and the returned function is discarded.
The usual intent is `defer cleanup()()`: `cleanup` runs at the defer statement,
and the returned function runs at the return.

The rule reports a deferred call whose result type is a function type. A
defined function type is reported too.

The rule leaves alone:

- a call that returns no result or a result of another type;
- a call with more than one result, even when one result is a function,
  because its result is a tuple and a multiple-value expression cannot be the
  callee of another call;
- a call inside a generic function whose result type is still a type
  parameter.

Fixable: the fix appends a call for each function in the result chain that can
be called with no arguments, so `defer cleanup()` becomes
`defer cleanup()()`. A function with no parameters and a function whose only
parameter is variadic can be called with no arguments. If the fix reaches a
result type it has already seen, the report has no fix. When the first
returned function cannot be called with no arguments, the report has no fix,
because no value is available to pass. The fix stops at a later function that
cannot be called with no arguments, and the next run reports the remaining
deferred call without a fix.

The rule is adapted from staticcheck's SA9010.

## Usage

```sh
go get -tool lesiw.io/tools/analyzers/deferchain/cmd/deferchain
go tool deferchain ./...
```

Or run it as part of the suite:

```sh
go tool lesiw.io/tools/cmd/vet -deferchain ./...
go tool lesiw.io/tools/cmd/fix -deferchain ./...
```
